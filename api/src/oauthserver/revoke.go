package oauthserver

import (
	"net/http"

	"github.com/a-digi/coco-iam/src/oauthserver/entity"
)

// RevokeHandler serves POST /oauth/revoke per RFC 7009. The
// caller authenticates as the client (client_id + secret in the
// POST body), then names a token to revoke. The server returns
// 200 OK regardless of whether the token actually existed —
// per the RFC, this avoids leaking token-existence info.
type RevokeHandler struct {
	ApplicationIDFromRequest func(r *http.Request) (applicationID, organizationID string, err error)
	Clients                  ClientRegistry
	Refresh                  RefreshStore
}

// @Summary		OAuth revoke endpoint
// @Description	Revokes a token. Returns 200 regardless of whether the token existed (per RFC 7009).
// @Tags			oauth
// @Accept			application/x-www-form-urlencoded
// @Produce		json
// @Param			client_id		formData	string	true	"Client ID"
// @Param			client_secret	formData	string	false	"Client secret"
// @Param			token			formData	string	true	"Token to revoke"
// @Param			token_type_hint	formData	string	false	"Hint: access_token or refresh_token"
// @Success		200
// @Failure		401	{object}	entity.OAuthErrorResponse
// @Router			/{orgSlug}/{wsSlug}/{appSlug}/oauth/revoke [post]
func (h *RevokeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Clients == nil || h.Refresh == nil || h.ApplicationIDFromRequest == nil {
		writeJSONError(w, entity.ErrCodeServerError, "revoke handler not configured", http.StatusInternalServerError)
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
	if token == "" {
		writeJSONError(w, entity.ErrCodeInvalidRequest, "token required", http.StatusBadRequest)
		return
	}
	// token_type_hint is purely advisory; we attempt refresh
	// revocation in any case — RFC 7009 §2.1.
	_ = h.Refresh.Revoke(r.Context(), token)
	// Access tokens are stateless JWTs in our model; we can't
	// revoke them server-side. The spec allows the server to
	// silently ignore unsupported token types.
	w.WriteHeader(http.StatusOK)
}
