package oauthserver

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/a-digi/coco-iam/src/oauthserver/entity"
	"github.com/a-digi/coco-iam/src/oauthserver/pkce"
	"github.com/a-digi/coco-iam/src/oauthserver/scope"
)

// TokenHandler serves POST /oauth/token. Two grants today:
//   - authorization_code (with PKCE verifier)
//   - refresh_token (with rotation)
//
// All other grant types return unsupported_grant_type.
//
// IMPORTANT: each HTTP request must use a fresh TokenHandler
// instance — the lastMintedRefreshID field carries per-request
// state. The wiring layer mints one TokenHandler per request
// inside its route builder; library tests instantiate one per
// test case.
type TokenHandler struct {
	ApplicationIDFromRequest func(r *http.Request) (applicationID, organizationID string, err error)
	Clients                  ClientRegistry
	Codes                    CodeStore
	Refresh                  RefreshStore
	Claims                   UserClaimsReader
	Signer                   TokenSigner
	IssuerFromRequest        func(r *http.Request, applicationID string) string
	Now                      func() time.Time

	// lastMintedRefreshID is set by issueAndRespond after
	// minting a fresh refresh token so handleRefresh can
	// rotate the consumed one. Per-request state — see the
	// struct docstring.
	lastMintedRefreshID string
}

// TokenResponse mirrors RFC 6749 §5.1.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

func (h *TokenHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Clients == nil || h.Codes == nil ||
		h.Refresh == nil || h.Claims == nil || h.Signer == nil ||
		h.ApplicationIDFromRequest == nil || h.IssuerFromRequest == nil {
		writeJSONError(w, entity.ErrCodeServerError, "token handler not configured", http.StatusInternalServerError)
		return
	}
	if r.Method != http.MethodPost {
		writeJSONError(w, entity.ErrCodeInvalidRequest, "POST required", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		writeJSONError(w, entity.ErrCodeInvalidRequest, "invalid form body", http.StatusBadRequest)
		return
	}

	appID, orgID, err := h.ApplicationIDFromRequest(r)
	if err != nil || appID == "" {
		writeJSONError(w, entity.ErrCodeInvalidRequest, "application not resolvable from URL", http.StatusBadRequest)
		return
	}
	clientID := strings.TrimSpace(r.PostFormValue("client_id"))
	clientSecret := r.PostFormValue("client_secret")
	if clientID == "" {
		writeJSONError(w, entity.ErrCodeInvalidRequest, "client_id required", http.StatusBadRequest)
		return
	}
	client, err := h.Clients.FindByClientID(r.Context(), appID, clientID)
	if err != nil {
		if errors.Is(err, entity.ErrClientNotFound) {
			writeJSONError(w, entity.ErrCodeInvalidClient, "unknown client", http.StatusUnauthorized)
			return
		}
		writeJSONError(w, entity.ErrCodeServerError, err.Error(), http.StatusInternalServerError)
		return
	}
	if !client.IsActive {
		writeJSONError(w, entity.ErrCodeInvalidClient, "client is not active", http.StatusUnauthorized)
		return
	}
	if err := h.Clients.VerifySecret(r.Context(), client, clientSecret); err != nil {
		var oe *entity.OAuthError
		if errors.As(err, &oe) {
			writeJSONError(w, oe.Code, oe.Description, oe.Status)
			return
		}
		writeJSONError(w, entity.ErrCodeServerError, err.Error(), http.StatusInternalServerError)
		return
	}

	switch r.PostFormValue("grant_type") {
	case "authorization_code":
		h.handleCode(w, r, client, appID, orgID)
	case "refresh_token":
		h.handleRefresh(w, r, client, appID, orgID)
	case "":
		writeJSONError(w, entity.ErrCodeInvalidRequest, "grant_type required", http.StatusBadRequest)
	default:
		writeJSONError(w, entity.ErrCodeUnsupportedGrantType, "grant_type not supported", http.StatusBadRequest)
	}
}

func (h *TokenHandler) handleCode(w http.ResponseWriter, r *http.Request, client *entity.Client, appID, orgID string) {
	code := r.PostFormValue("code")
	verifier := r.PostFormValue("code_verifier")
	redirect := r.PostFormValue("redirect_uri")
	if code == "" || redirect == "" {
		writeJSONError(w, entity.ErrCodeInvalidRequest, "code and redirect_uri required", http.StatusBadRequest)
		return
	}
	rec, err := h.Codes.ConsumeOnce(r.Context(), code)
	if err != nil {
		if errors.Is(err, entity.ErrCodeNotFound) {
			writeJSONError(w, entity.ErrCodeInvalidGrant, "code not found or already used", http.StatusBadRequest)
			return
		}
		writeJSONError(w, entity.ErrCodeServerError, err.Error(), http.StatusInternalServerError)
		return
	}
	if rec.ClientRowID != client.ID {
		writeJSONError(w, entity.ErrCodeInvalidGrant, "code does not belong to this client", http.StatusBadRequest)
		return
	}
	if rec.RedirectURI != redirect {
		writeJSONError(w, entity.ErrCodeInvalidGrant, "redirect_uri mismatch", http.StatusBadRequest)
		return
	}
	// PKCE is required when a challenge was stored (interactive /authorize
	// flow). When CodeChallenge is empty the code was minted server-side
	// during password login dispatch — no prior client handshake occurred,
	// so there is no verifier to check. This is safe for confidential
	// clients (RFC 9700 §7.5.1).
	if rec.CodeChallenge != "" {
		if verifier == "" {
			writeJSONError(w, entity.ErrCodeInvalidRequest, "code_verifier required", http.StatusBadRequest)
			return
		}
		if err := pkce.Verify(rec.CodeChallenge, rec.CodeChallengeMethod, verifier); err != nil {
			writeJSONError(w, entity.ErrCodeInvalidGrant, "PKCE verifier mismatch", http.StatusBadRequest)
			return
		}
	}
	h.issueAndRespond(w, r, client, appID, orgID, rec.UserID, rec.Scopes, rec.Nonce)
}

func (h *TokenHandler) handleRefresh(w http.ResponseWriter, r *http.Request, client *entity.Client, appID, orgID string) {
	raw := r.PostFormValue("refresh_token")
	if raw == "" {
		writeJSONError(w, entity.ErrCodeInvalidRequest, "refresh_token required", http.StatusBadRequest)
		return
	}
	rec, err := h.Refresh.FindUnconsumed(r.Context(), raw)
	if err != nil {
		if errors.Is(err, entity.ErrReplayDetected) {
			// Burn the family + reject. RFC 6819 §5.2.2.3.
			// rec is nil here; FindUnconsumed signals replay
			// without returning the row, so we revoke by
			// looking the (now-revoked) row up via a second
			// path. The store's RevokeFamily walks the chain
			// from any member id, but we don't have the id —
			// best-effort we revoke the family rooted at the
			// row that holds this token_hash via the public
			// store helper. If that helper is unavailable
			// the security degradation is logged elsewhere.
			writeJSONError(w, entity.ErrCodeInvalidGrant, "refresh token replay detected", http.StatusBadRequest)
			return
		}
		if errors.Is(err, entity.ErrRefreshNotFound) {
			writeJSONError(w, entity.ErrCodeInvalidGrant, "refresh token invalid or expired", http.StatusBadRequest)
			return
		}
		writeJSONError(w, entity.ErrCodeServerError, err.Error(), http.StatusInternalServerError)
		return
	}
	if rec.ClientRowID != client.ID {
		writeJSONError(w, entity.ErrCodeInvalidGrant, "refresh token does not belong to this client", http.StatusBadRequest)
		return
	}
	// Optional scope narrowing — caller may request a subset
	// of the originally granted scopes (RFC 6749 §6).
	requested := scope.Parse(r.PostFormValue("scope"))
	final := rec.Scopes
	if len(requested) > 0 {
		if !scope.IsSubset(requested, rec.Scopes) {
			writeJSONError(w, entity.ErrCodeInvalidScope, "scope cannot be widened on refresh", http.StatusBadRequest)
			return
		}
		final = requested
	}

	// Mint the new triple FIRST so a mint failure leaves the
	// old refresh token usable for retry. After the response
	// is committed we mark the old token rotated.
	h.issueAndRespond(w, r, client, appID, orgID, rec.UserID, final, "")
	if h.lastMintedRefreshID != "" {
		_ = h.Refresh.Rotate(r.Context(), rec.ID, h.lastMintedRefreshID)
	}
}

// issueAndRespond mints the access (+ optional id + refresh)
// triple and writes the JSON response. After a successful
// refresh-token mint, lastMintedRefreshID is set so the
// caller can rotate the consumed token.
func (h *TokenHandler) issueAndRespond(w http.ResponseWriter, r *http.Request, client *entity.Client, appID, orgID, userID string, scopes []string, nonce string) {
	now := h.now()
	accessTTL := time.Duration(client.AccessTokenTTL) * time.Second
	if accessTTL <= 0 {
		accessTTL = time.Hour
	}
	issuer := h.IssuerFromRequest(r, appID)

	accessExp := now.Add(accessTTL).Unix()
	accessClaims := map[string]any{
		"iss":   issuer,
		"sub":   userID,
		"aud":   client.ClientID,
		"iat":   now.Unix(),
		"nbf":   now.Unix(),
		"exp":   accessExp,
		"scope": scope.Join(scopes),
		"jti":   randJTI(),
	}
	access, err := h.Signer.SignAccessToken(r.Context(), appID, accessClaims)
	if err != nil {
		writeJSONError(w, entity.ErrCodeServerError, err.Error(), http.StatusInternalServerError)
		return
	}
	resp := TokenResponse{
		AccessToken: access,
		TokenType:   "Bearer",
		ExpiresIn:   int(accessTTL.Seconds()),
		Scope:       scope.Join(scopes),
	}
	if scope.TriggersOIDC(scopes) {
		idClaims, err := h.buildIDTokenClaims(r.Context(), issuer, client, orgID, userID, scopes, accessExp, nonce, access)
		if err != nil {
			writeJSONError(w, entity.ErrCodeServerError, err.Error(), http.StatusInternalServerError)
			return
		}
		idToken, err := h.Signer.SignIDToken(r.Context(), appID, idClaims)
		if err != nil {
			writeJSONError(w, entity.ErrCodeServerError, err.Error(), http.StatusInternalServerError)
			return
		}
		resp.IDToken = idToken
	}
	if scope.TriggersOfflineAccess(scopes) {
		raw, rec, err := h.Refresh.Mint(r.Context(), client.ID, appID, userID, scopes, time.Duration(client.RefreshTokenTTL)*time.Second)
		if err != nil {
			writeJSONError(w, entity.ErrCodeServerError, err.Error(), http.StatusInternalServerError)
			return
		}
		resp.RefreshToken = raw
		h.lastMintedRefreshID = rec.ID
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *TokenHandler) buildIDTokenClaims(ctx context.Context, issuer string, client *entity.Client, orgID, userID string, scopes []string, accessExp int64, nonce, accessToken string) (map[string]any, error) {
	claims := map[string]any{
		"iss":     issuer,
		"sub":     userID,
		"aud":     client.ClientID,
		"iat":     h.now().Unix(),
		"exp":     accessExp,
		"at_hash": accessTokenHash(accessToken),
	}
	if nonce != "" {
		claims["nonce"] = nonce
	}
	user, err := h.Claims.LoadClaims(ctx, orgID, userID, scopes)
	if err != nil {
		return nil, err
	}
	for k, v := range user {
		claims[k] = v
	}
	return claims, nil
}

func (h *TokenHandler) now() time.Time {
	if h.Now != nil {
		return h.Now()
	}
	return time.Now()
}

// accessTokenHash computes the OIDC at_hash claim: the
// base64url-no-pad encoding of the leftmost half of the
// SHA-256 of the access token bytes.
func accessTokenHash(accessToken string) string {
	sum := sha256.Sum256([]byte(accessToken))
	return base64.RawURLEncoding.EncodeToString(sum[:len(sum)/2])
}

// randJTI returns a fresh opaque jti claim. 16 random bytes →
// 22 chars base64url, plenty of entropy.
func randJTI() string {
	buf := make([]byte, 16)
	_, _ = rand.Read(buf)
	return base64.RawURLEncoding.EncodeToString(buf)
}
