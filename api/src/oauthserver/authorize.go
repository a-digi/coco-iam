package oauthserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/a-digi/coco-iam/src/oauthserver/entity"
	"github.com/a-digi/coco-iam/src/oauthserver/scope"
)

// AuthorizeRequest is the parsed shape of an /authorize call.
// Fields map 1:1 to RFC 6749 §4.1.1 + RFC 7636. The handler
// uses ParseAuthorizeRequest to populate this from a raw URL
// query so the validation rules sit next to the field
// definitions.
type AuthorizeRequest struct {
	ClientID            string
	RedirectURI         string
	ResponseType        string
	Scope               string
	State               string
	Nonce               string
	CodeChallenge       string
	CodeChallengeMethod string
}

// ParseAuthorizeRequest reads the request from a raw URL query
// (already parsed via url.Values). Returns a structured value;
// validation happens in ValidateAuthorizeRequest so callers can
// surface field-level errors precisely.
func ParseAuthorizeRequest(q url.Values) AuthorizeRequest {
	return AuthorizeRequest{
		ClientID:            strings.TrimSpace(q.Get("client_id")),
		RedirectURI:         strings.TrimSpace(q.Get("redirect_uri")),
		ResponseType:        strings.TrimSpace(q.Get("response_type")),
		Scope:               strings.TrimSpace(q.Get("scope")),
		State:               strings.TrimSpace(q.Get("state")),
		Nonce:               strings.TrimSpace(q.Get("nonce")),
		CodeChallenge:       strings.TrimSpace(q.Get("code_challenge")),
		CodeChallengeMethod: strings.TrimSpace(q.Get("code_challenge_method")),
	}
}

// AuthorizeDecision is the output of the validation +
// scope-resolution layer. The handler uses it to render the
// consent screen, the redirect, or an error.
type AuthorizeDecision struct {
	Client            *entity.Client
	GrantedScopes     []string  // intersection of requested + allowed
	NeedsConsent      bool      // true → render screen; false → mint code immediately
	ConsentRecord     *entity.Consent
}

// ValidateAuthorizeRequest runs the synchronous validation
// pipeline: client lookup, redirect_uri match, response_type
// support, PKCE presence, scope filtering. On success it
// populates an AuthorizeDecision the handler can act on.
//
// On failure it returns either:
//   - an OAuthError (suitable to either redirect the user back
//     to the client with `error=...` or, for redirect_uri /
//     client_id failures, render server-side because the
//     redirect target itself is untrusted), OR
//   - a transport error (DB down, etc.) the handler maps to
//     server_error.
func ValidateAuthorizeRequest(ctx context.Context, applicationID string, req AuthorizeRequest, registry ClientRegistry) (*AuthorizeDecision, error) {
	if req.ClientID == "" {
		return nil, entity.NewOAuthError(entity.ErrCodeInvalidRequest, "client_id is required", http.StatusBadRequest)
	}
	if req.RedirectURI == "" {
		return nil, entity.NewOAuthError(entity.ErrCodeInvalidRequest, "redirect_uri is required", http.StatusBadRequest)
	}
	client, err := registry.FindByClientID(ctx, applicationID, req.ClientID)
	if err != nil {
		if errors.Is(err, entity.ErrClientNotFound) {
			return nil, entity.NewOAuthError(entity.ErrCodeInvalidClient, "unknown client", http.StatusBadRequest)
		}
		return nil, err
	}
	if !client.IsActive {
		return nil, entity.NewOAuthError(entity.ErrCodeInvalidClient, "client is not active", http.StatusBadRequest)
	}
	if !client.PermitsRedirect(req.RedirectURI) {
		return nil, entity.NewOAuthError(entity.ErrCodeInvalidRequest, "redirect_uri does not match a registered value", http.StatusBadRequest)
	}
	// response_type — only "code" is supported in MVP.
	if req.ResponseType != "code" {
		return nil, entity.NewOAuthError(entity.ErrCodeUnsupportedResponseType, "only response_type=code is supported", http.StatusBadRequest)
	}
	// PKCE required for public clients; we still require it
	// for confidential clients per modern best practice.
	if strings.TrimSpace(req.CodeChallenge) == "" {
		return nil, entity.NewOAuthError(entity.ErrCodeInvalidRequest, "code_challenge is required (PKCE)", http.StatusBadRequest)
	}
	method := req.CodeChallengeMethod
	if method == "" {
		method = "S256"
	}
	if method != "S256" {
		return nil, entity.NewOAuthError(entity.ErrCodeInvalidRequest, "only code_challenge_method=S256 is supported", http.StatusBadRequest)
	}
	requested := scope.Parse(req.Scope)
	granted := scope.FilterAllowed(requested, client.AllowedScopes)
	if len(requested) > 0 && len(granted) == 0 {
		return nil, entity.NewOAuthError(entity.ErrCodeInvalidScope, "no requested scope is allowed for this client", http.StatusBadRequest)
	}
	return &AuthorizeDecision{
		Client:        client,
		GrantedScopes: granted,
		NeedsConsent:  client.RequireConsent,
	}, nil
}

// ResolveConsent layers the consent-cache check on top of an
// AuthorizeDecision. Loads the user's prior consent (if any)
// and, when the cached decision covers the requested scope set,
// flips NeedsConsent to false so the handler skips the screen.
//
// A miss / revoked consent leaves NeedsConsent at whatever the
// client config requires — RequireConsent=false implies
// "auto-approve every time", which the validation layer already
// captured.
func ResolveConsent(ctx context.Context, organizationID, userID string, decision *AuthorizeDecision, consents ConsentStore) error {
	if !decision.NeedsConsent {
		return nil
	}
	current, err := consents.Load(ctx, organizationID, userID, decision.Client.ID)
	if err != nil {
		if errors.Is(err, entity.ErrConsentNotFound) {
			return nil
		}
		return err
	}
	decision.ConsentRecord = current
	if scope.IsSubset(decision.GrantedScopes, current.GrantedScopes) {
		decision.NeedsConsent = false
	}
	return nil
}

// AuthorizeHandler stitches the parse → validate → user-auth →
// consent-decide → mint-code flow together. It depends only on
// the ports defined in this package — no coco-iam types leak
// in.
type AuthorizeHandler struct {
	// ApplicationIDFromRequest extracts the per-request
	// application id from the URL. Wiring closes over the
	// slug-resolver. Keeping it as a callback avoids dragging
	// a slug interface into the library.
	ApplicationIDFromRequest func(r *http.Request) (applicationID, organizationID string, err error)
	Clients                  ClientRegistry
	Codes                    CodeStore
	Consents                 ConsentStore
	Auth                     UserAuthenticator
	// LoginRedirectURL builds the URL to send unauthenticated
	// users to. Caller provides this so the library doesn't
	// know about coco-iam routes.
	LoginRedirectURL func(r *http.Request, returnTo string) string
	// RenderConsent renders the consent screen as HTML. Wiring
	// supplies a function that uses html/template against the
	// project's standard layout. Library-only fallback prints
	// a plain JSON form so tests can drive the consent POST.
	RenderConsent func(w http.ResponseWriter, params ConsentRenderParams)
	// CodeTTL bounds how long the issued authorization code
	// stays valid. 0 → 5 minutes.
	CodeTTL time.Duration
	// ScopeEnricher, when non-nil, is called just before the code
	// is minted so the caller can append extra scopes (e.g. ACL
	// roles for organisation users). Must return a new slice —
	// never mutate the input. Returning nil is treated as "no
	// change".
	ScopeEnricher func(ctx context.Context, appID, userID string, granted []string) []string
}

// ConsentRenderParams is the data the wiring layer's
// renderConsent template needs. Kept minimal and json-tagged so
// the default fallback can encode it directly.
type ConsentRenderParams struct {
	ClientDisplayName string   `json:"client_display_name"`
	ClientID          string   `json:"client_id"`
	RequestedScopes   []string `json:"requested_scopes"`
	// FormAction is the URL the consent form posts back to
	// (typically the same /authorize endpoint with method=POST
	// + an `approve=yes` field).
	FormAction string `json:"form_action"`
	// State + ReturnURL echo through the consent form so the
	// POST handler can resume the original flow.
	State     string `json:"state"`
	ReturnURL string `json:"return_url"`
}

// ServeHTTP runs the full authorize pipeline.
func (h *AuthorizeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Clients == nil || h.Codes == nil ||
		h.Consents == nil || h.Auth == nil || h.ApplicationIDFromRequest == nil {
		writeJSONError(w, entity.ErrCodeServerError, "authorize handler not configured", http.StatusInternalServerError)
		return
	}

	appID, orgID, err := h.ApplicationIDFromRequest(r)
	if err != nil || appID == "" {
		writeJSONError(w, entity.ErrCodeInvalidRequest, "application not resolvable from URL", http.StatusBadRequest)
		return
	}

	req := ParseAuthorizeRequest(r.URL.Query())
	ctx := r.Context()
	decision, err := ValidateAuthorizeRequest(ctx, appID, req, h.Clients)
	if err != nil {
		h.surfaceError(w, r, &req, err)
		return
	}

	// Auth check — does the caller already have a session?
	userID, sessionOrgID, err := h.Auth.CurrentUser(ctx, requestInfoFrom(r))
	if err != nil {
		writeJSONError(w, entity.ErrCodeServerError, err.Error(), http.StatusInternalServerError)
		return
	}
	if userID == "" {
		// Not logged in → bounce to the login page, telling
		// it where to come back to once the session is
		// established.
		returnTo := r.URL.String()
		if h.LoginRedirectURL == nil {
			writeJSONError(w, entity.ErrCodeServerError, "no login redirect configured", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, h.LoginRedirectURL(r, returnTo), http.StatusFound)
		return
	}

	// Cross-org check: a user signed in to one org's session
	// cookie can't authorize a client registered under another
	// org's application.
	if sessionOrgID != "" && orgID != "" && sessionOrgID != orgID {
		h.surfaceError(w, r, &req,
			entity.NewOAuthError(entity.ErrCodeAccessDenied, "session belongs to a different organisation", http.StatusForbidden))
		return
	}

	// Consent check.
	if err := ResolveConsent(ctx, orgID, userID, decision, h.Consents); err != nil {
		writeJSONError(w, entity.ErrCodeServerError, err.Error(), http.StatusInternalServerError)
		return
	}

	// POST → consent submission.
	if r.Method == http.MethodPost {
		if r.URL.Query().Get("approve") != "yes" && r.FormValue("approve") != "yes" {
			h.surfaceError(w, r, &req,
				entity.NewOAuthError(entity.ErrCodeAccessDenied, "user denied the authorization request", http.StatusForbidden))
			return
		}
		if err := h.Consents.Record(ctx, orgID, userID, decision.Client.ID, decision.GrantedScopes); err != nil {
			writeJSONError(w, entity.ErrCodeServerError, err.Error(), http.StatusInternalServerError)
			return
		}
		h.issueCodeAndRedirect(w, r, ctx, appID, userID, decision, &req)
		return
	}

	// GET, no consent needed (auto-approved or cached).
	if !decision.NeedsConsent {
		h.issueCodeAndRedirect(w, r, ctx, appID, userID, decision, &req)
		return
	}

	// GET, consent needed → render the screen.
	render := h.RenderConsent
	if render == nil {
		render = defaultJSONConsentRender
	}
	render(w, ConsentRenderParams{
		ClientDisplayName: decision.Client.DisplayName,
		ClientID:          decision.Client.ClientID,
		RequestedScopes:   decision.GrantedScopes,
		FormAction:        r.URL.Path + "?" + r.URL.RawQuery,
		State:             req.State,
		ReturnURL:         r.URL.String(),
	})
}

func (h *AuthorizeHandler) issueCodeAndRedirect(w http.ResponseWriter, r *http.Request, ctx context.Context, appID, userID string, decision *AuthorizeDecision, req *AuthorizeRequest) {
	scopes := decision.GrantedScopes
	if h.ScopeEnricher != nil {
		if enriched := h.ScopeEnricher(ctx, appID, userID, scopes); enriched != nil {
			scopes = enriched
		}
	}
	code, err := h.Codes.Mint(ctx, CodeMintInput{
		ClientRowID:         decision.Client.ID,
		ApplicationID:       appID,
		UserID:              userID,
		RedirectURI:         req.RedirectURI,
		Scopes:              scopes,
		CodeChallenge:       req.CodeChallenge,
		CodeChallengeMethod: req.CodeChallengeMethod,
		Nonce:               req.Nonce,
	}, h.CodeTTL)
	if err != nil {
		writeJSONError(w, entity.ErrCodeServerError, err.Error(), http.StatusInternalServerError)
		return
	}
	target := buildRedirect(req.RedirectURI, map[string]string{
		"code":  code,
		"state": req.State,
	})
	http.Redirect(w, r, target, http.StatusFound)
}

// surfaceError redirects an OAuthError back to the client when
// a redirect_uri is known and trustworthy; otherwise renders a
// plain JSON error so the user isn't sent to an attacker-
// controlled URI.
func (h *AuthorizeHandler) surfaceError(w http.ResponseWriter, r *http.Request, req *AuthorizeRequest, err error) {
	var oe *entity.OAuthError
	if !errors.As(err, &oe) {
		writeJSONError(w, entity.ErrCodeServerError, err.Error(), http.StatusInternalServerError)
		return
	}
	// Redirect-back is only safe when the redirect_uri has
	// been validated against a registered client; the
	// invalid_client / invalid_request branches that fail
	// before lookup have unverified redirect_uris and must
	// stay server-rendered.
	switch oe.Code {
	case entity.ErrCodeInvalidClient, entity.ErrCodeInvalidRequest, entity.ErrCodeServerError:
		writeJSONError(w, oe.Code, oe.Description, oe.Status)
		return
	}
	target := buildRedirect(req.RedirectURI, map[string]string{
		"error":             string(oe.Code),
		"error_description": oe.Description,
		"state":             req.State,
	})
	http.Redirect(w, r, target, http.StatusFound)
}

// requestInfoFrom is the trivial conversion from *http.Request
// to the narrow RequestInfo the UserAuthenticator interface
// uses. Keeps the library out of net/http types in its public
// ports surface.
func requestInfoFrom(r *http.Request) RequestInfo {
	cookieName := "coco_iam_auth"
	cookieVal := ""
	if c, err := r.Cookie(cookieName); err == nil {
		cookieVal = c.Value
	}
	return RequestInfo{
		CookieValue: cookieVal,
		Header:      r.Header.Get("Authorization"),
	}
}

// buildRedirect appends the params to the redirect URI as a
// query string. Preserves any existing query.
func buildRedirect(base string, params map[string]string) string {
	u, err := url.Parse(base)
	if err != nil {
		// Bad URL would have been rejected at validation —
		// returning the base verbatim isn't helpful but is
		// safer than crashing.
		return base
	}
	q := u.Query()
	for k, v := range params {
		if v != "" {
			q.Set(k, v)
		}
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// writeJSONError emits an OAuth-spec error envelope.
// Used for server-rendered failures (no client redirect).
func writeJSONError(w http.ResponseWriter, code entity.OAuthErrorCode, description string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error":             string(code),
		"error_description": description,
	})
}

// defaultJSONConsentRender is the fallback when no template is
// wired up. Encodes the params + the literal HTML form snippet
// so test harnesses can submit the form. Production wiring
// always supplies a real renderer.
func defaultJSONConsentRender(w http.ResponseWriter, p ConsentRenderParams) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"consent": p,
		// Echoed plain-text instructions for tests to drive
		// the next call.
		"submit_to": fmt.Sprintf("%s&approve=yes", p.FormAction),
	})
}
