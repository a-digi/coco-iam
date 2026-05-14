package oauthserver

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/a-digi/coco-iam/src/oauthserver/entity"
)

// UserinfoHandler serves GET /oauth/userinfo. Authenticates
// the caller via the bearer access token, validates it via the
// supplied AccessTokenVerifier, and returns the OIDC claim set
// for the user the token belongs to.
//
// Filtering: the response includes only claims permitted by
// the scope set in the access token (per OIDC §5.4).
type UserinfoHandler struct {
	ApplicationIDFromRequest func(r *http.Request) (applicationID, organizationID string, err error)
	Verifier                 AccessTokenVerifier
	Claims                   UserClaimsReader
}

// AccessTokenVerifier is the seam between the userinfo handler
// and the per-application key service. Production wiring
// supplies an implementation that calls the existing
// keys.Service.LoadVerifiablePublicKey + verifies the JWT.
//
// Returns the user id + scope list on success. Any failure
// (expired token, bad signature, wrong audience) returns
// ErrAccessTokenInvalid. Returning a typed error keeps the
// handler's error-mapping table tiny.
type AccessTokenVerifier interface {
	VerifyAccessToken(ctx context.Context, applicationID, bearerToken string) (userID string, scopes []string, err error)
}

// ErrAccessTokenInvalid is the typed error AccessTokenVerifier
// implementations return when the bearer is unusable for any
// reason. Userinfo maps it to 401.
var ErrAccessTokenInvalid = &entity.OAuthError{
	Code:        entity.ErrCodeInvalidRequest,
	Description: "access token invalid or expired",
	Status:      http.StatusUnauthorized,
}

func (h *UserinfoHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Verifier == nil || h.Claims == nil || h.ApplicationIDFromRequest == nil {
		writeJSONError(w, entity.ErrCodeServerError, "userinfo handler not configured", http.StatusInternalServerError)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		writeJSONError(w, entity.ErrCodeInvalidRequest, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	bearer := bearerFromRequest(r)
	if bearer == "" {
		w.Header().Set("WWW-Authenticate", `Bearer realm="userinfo"`)
		writeJSONError(w, entity.ErrCodeInvalidRequest, "bearer token required", http.StatusUnauthorized)
		return
	}
	appID, orgID, err := h.ApplicationIDFromRequest(r)
	if err != nil || appID == "" {
		writeJSONError(w, entity.ErrCodeInvalidRequest, "application not resolvable from URL", http.StatusBadRequest)
		return
	}
	userID, scopes, err := h.Verifier.VerifyAccessToken(r.Context(), appID, bearer)
	if err != nil {
		writeJSONError(w, entity.ErrCodeInvalidRequest, "access token invalid or expired", http.StatusUnauthorized)
		return
	}
	all, err := h.Claims.LoadClaims(r.Context(), orgID, userID, scopes)
	if err != nil {
		writeJSONError(w, entity.ErrCodeServerError, err.Error(), http.StatusInternalServerError)
		return
	}
	out := map[string]any{"sub": userID}
	for k, v := range all {
		out[k] = v
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(out)
}

// bearerFromRequest plucks the bearer token from the
// Authorization header. Strict parsing — leading whitespace
// only, exactly one "Bearer " prefix.
func bearerFromRequest(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if h == "" {
		return ""
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(h, prefix) {
		return ""
	}
	return strings.TrimSpace(h[len(prefix):])
}
