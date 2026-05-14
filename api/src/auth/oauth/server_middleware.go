package oauth

import (
	"net/http"

	oauth "github.com/a-digi/coco-oauth/oauth"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/routing"
)

type protectedServerHandler struct {
	inner     routing.HandlerInterface
	validator oauth.TokenValidator
	required  []string
}

func (h *protectedServerHandler) ServeHTTP(reqCtx request.RequestContext) {
	if h.validator == nil {
		reqCtx.Status(http.StatusUnauthorized)
		return
	}
	authHeader := reqCtx.GetRequest().Header.Get("Authorization")
	token, err := oauth.ExtractBearer(authHeader)
	if err != nil {
		reqCtx.Status(http.StatusUnauthorized)
		return
	}
	sub, scopes, _, err := h.validator.Validate(token)
	if err != nil || sub == "" {
		reqCtx.Status(http.StatusUnauthorized)
		return
	}

	if !oauth.HasAllScopes(scopes, h.required) {
		reqCtx.Status(http.StatusForbidden)
		return
	}

	h.inner.ServeHTTP(reqCtx)
}

// WrapServerHandler creates a new handler with bearer auth + scope requirements.
func WrapServerHandler(inner routing.HandlerInterface, validator oauth.TokenValidator, requiredScopes []string) routing.HandlerInterface {
	return &protectedServerHandler{inner: inner, validator: validator, required: requiredScopes}
}
