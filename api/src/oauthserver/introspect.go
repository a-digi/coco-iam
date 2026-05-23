package oauthserver

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/a-digi/coco-iam/src/oauthserver/entity"
	"github.com/a-digi/coco-iam/src/oauthserver/scope"
	"github.com/a-digi/coco-iam/src/oauthserver/tokenid"
)

// IntrospectHandler serves POST /oauth/introspect per RFC 7662.
// Resource servers POST a token + their client credentials and
// receive `{active: true, ...claims}` for currently-valid
// tokens, or `{active: false}` for anything else (per the RFC,
// we never leak why a token is inactive).
type IntrospectHandler struct {
	ApplicationIDFromRequest func(r *http.Request) (applicationID, organizationID string, err error)
	Clients                  ClientRegistry
	Refresh                  RefreshStore
	Verifier                 AccessTokenVerifier
}

// IntrospectionResponse mirrors RFC 7662 §2.2.
type IntrospectionResponse struct {
	Active   bool   `json:"active"`
	Scope    string `json:"scope,omitempty"`
	ClientID string `json:"client_id,omitempty"`
	TokenType string `json:"token_type,omitempty"`
	Sub      string `json:"sub,omitempty"`
	Exp      int64  `json:"exp,omitempty"`
}

// @Summary		OAuth introspect endpoint
// @Description	Validates a token and returns its claims. Returns {active:false} for any invalid/expired token.
// @Tags			oauth
// @Accept			application/x-www-form-urlencoded
// @Produce		json
// @Param			client_id		formData	string	true	"Client ID"
// @Param			client_secret	formData	string	true	"Client secret"
// @Param			token			formData	string	true	"Token to introspect"
// @Param			token_type_hint	formData	string	false	"Hint: access_token or refresh_token"
// @Success		200	{object}	IntrospectionResponse
// @Failure		401	{object}	entity.OAuthErrorResponse
// @Router			/{orgSlug}/{wsSlug}/{appSlug}/oauth/introspect [post]
func (h *IntrospectHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Clients == nil || h.Refresh == nil ||
		h.Verifier == nil || h.ApplicationIDFromRequest == nil {
		writeJSONError(w, entity.ErrCodeServerError, "introspect handler not configured", http.StatusInternalServerError)
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
	appID, _, err := h.ApplicationIDFromRequest(r)
	if err != nil || appID == "" {
		writeJSONError(w, entity.ErrCodeInvalidRequest, "application not resolvable", http.StatusBadRequest)
		return
	}
	clientID := r.PostFormValue("client_id")
	if clientID == "" {
		writeJSONError(w, entity.ErrCodeInvalidClient, "client_id required", http.StatusUnauthorized)
		return
	}
	client, err := h.Clients.FindByClientID(r.Context(), appID, clientID)
	if err != nil {
		writeJSONError(w, entity.ErrCodeInvalidClient, "unknown client", http.StatusUnauthorized)
		return
	}
	if err := h.Clients.VerifySecret(r.Context(), client, r.PostFormValue("client_secret")); err != nil {
		writeJSONError(w, entity.ErrCodeInvalidClient, "client authentication failed", http.StatusUnauthorized)
		return
	}
	token := r.PostFormValue("token")
	hint := r.PostFormValue("token_type_hint")
	if token == "" {
		writeJSON(w, IntrospectionResponse{Active: false})
		return
	}
	// Try refresh first when hint says so; else access first.
	if hint == "refresh_token" {
		if resp, ok := h.tryRefresh(r, client, token); ok {
			writeJSON(w, resp)
			return
		}
		if resp, ok := h.tryAccess(r, appID, client, token); ok {
			writeJSON(w, resp)
			return
		}
	} else {
		if resp, ok := h.tryAccess(r, appID, client, token); ok {
			writeJSON(w, resp)
			return
		}
		if resp, ok := h.tryRefresh(r, client, token); ok {
			writeJSON(w, resp)
			return
		}
	}
	// Unknown / inactive token — RFC 7662 §2.2: respond with
	// `{active: false}` and no other claims.
	writeJSON(w, IntrospectionResponse{Active: false})
}

func (h *IntrospectHandler) tryAccess(r *http.Request, appID string, client *entity.Client, token string) (IntrospectionResponse, bool) {
	userID, scopes, err := h.Verifier.VerifyAccessToken(r.Context(), appID, token)
	if err != nil {
		return IntrospectionResponse{}, false
	}
	return IntrospectionResponse{
		Active:    true,
		Scope:     scope.Join(scopes),
		ClientID:  client.ClientID,
		TokenType: "Bearer",
		Sub:       userID,
	}, true
}

func (h *IntrospectHandler) tryRefresh(r *http.Request, client *entity.Client, token string) (IntrospectionResponse, bool) {
	rec, err := h.Refresh.FindUnconsumed(r.Context(), token)
	if err != nil {
		// ErrReplayDetected → "active=false" per spec; we
		// don't reveal that the token was replayed here.
		if errors.Is(err, entity.ErrReplayDetected) || errors.Is(err, entity.ErrRefreshNotFound) {
			return IntrospectionResponse{}, false
		}
		return IntrospectionResponse{}, false
	}
	if rec.ClientRowID != client.ID {
		return IntrospectionResponse{}, false
	}
	return IntrospectionResponse{
		Active:    true,
		Scope:     scope.Join(rec.Scopes),
		ClientID:  client.ClientID,
		TokenType: "refresh_token",
		Sub:       rec.UserID,
	}, true
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(v)
}

// Compile-time guard so tokenid keeps a referenced symbol —
// future CRUD on cached introspection responses will reuse it.
var _ = tokenid.Hash
