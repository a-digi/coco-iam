package handler

import (
	"net/http"

	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// PublicJWKSHandler serves
// GET /api/v1/public/applications/{id}/.well-known/jwks.json.
// Returns a JWK Set with every currently-verifiable key — the
// active key plus any deactivated keys whose `expires_at` is still
// in the future. Downstream services re-fetch this to verify tokens
// signed by either the current or a recently-retired key.
type PublicJWKSHandler struct{}

// @Summary     Get public JWKS for an application
// @Tags        app-keys
// @Produce     json
// @Param       id path string true "Application ID"
// @Router      /applications/{id}/.well-known/jwks.json [get]
func (h *PublicJWKSHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()
	svc := resolveService(reqCtx)
	if svc == nil {
		return
	}
	appID := segmentAfter(r.URL.Path, "applications")
	if appID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "missing application id")
		return
	}
	keys, err := svc.VerifiableJWKS(appID)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, map[string]any{
		"keys": keys,
	})
}
