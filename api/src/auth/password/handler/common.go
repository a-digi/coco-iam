// Package handler exposes HTTP handlers for self-service password
// change. Separate from the service layer so the service stays
// transport-agnostic.
package handler

import (
	"net/http"

	"github.com/a-digi/coco-iam/src/auth/password"
	jwt_token "github.com/a-digi/coco-iam/src/auth/security/jwt"
	"github.com/a-digi/coco-oauth/oauth"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// genericFailureMsg — the only error string returned for any
// authentication-related password-change failure. Kept deliberately
// vague so the endpoint can't be used to test passwords or probe
// user existence.
const genericFailureMsg = "Something went wrong. Please try again."

type bagGetter interface {
	Get(key string) (interface{}, bool)
}

// resolveService fetches the Service from the DI bag. Writes a 500 and
// returns nil if the service isn't wired — handlers should early-return.
func resolveService(reqCtx request.RequestContext) *password.Service {
	w := reqCtx.GetWriter()
	ctx := reqCtx.GetDI()
	bag, ok := ctx.(bagGetter)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "DI context has unexpected type")
		return nil
	}
	raw, ok := bag.Get(password.ContextBagKeyService)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "password service not available")
		return nil
	}
	svc, ok := raw.(*password.Service)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "password service has unexpected type")
		return nil
	}
	return svc
}

// userIDFromRequest extracts the authenticated user ID from the Bearer
// token. Writes a 401 and returns "" on any failure.
func userIDFromRequest(reqCtx request.RequestContext) string {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		response.ErrorResponse(w, http.StatusUnauthorized, "unauthorized")
		return ""
	}
	token, err := oauth.ExtractBearer(authHeader)
	if err != nil {
		response.ErrorResponse(w, http.StatusUnauthorized, "invalid token format")
		return ""
	}
	userID, err := jwt_token.ParseJWTSubject(token)
	if err != nil || userID == "" {
		response.ErrorResponse(w, http.StatusUnauthorized, "failed to parse user from token")
		return ""
	}
	return userID
}
