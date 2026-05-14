// Package handler exposes HTTP handlers for the user-rules domain.
// Split from the package root so service-layer code stays
// transport-agnostic.
package handler

import (
	"net/http"

	jwt_token "github.com/a-digi/coco-iam/src/auth/security/jwt"
	"github.com/a-digi/coco-iam/src/userrules"
	"github.com/a-digi/coco-oauth/oauth"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

type bagGetter interface {
	Get(key string) (interface{}, bool)
}

func resolveStore(reqCtx request.RequestContext) *userrules.Store {
	w := reqCtx.GetWriter()
	ctx := reqCtx.GetDI()
	bag, ok := ctx.(bagGetter)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "DI context has unexpected type")
		return nil
	}
	raw, ok := bag.Get(userrules.ContextBagKeyStore)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "user-rules store not available")
		return nil
	}
	store, ok := raw.(*userrules.Store)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "user-rules store has unexpected type")
		return nil
	}
	return store
}

// userIDFromRequest extracts the authenticated user ID from the
// Bearer token. Writes a 401 and returns "" on failure.
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
