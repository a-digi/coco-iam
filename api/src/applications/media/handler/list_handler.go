package handler

import (
	"net/http"

	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// ListHandler serves GET /api/v1/applications/{res:applications}/{id:<appId>}/media.
// Accepts an optional `?parent_id=<id>` query param — when absent, lists the
// root of the application's media tree.
type ListHandler struct{}

// @Summary     List media for an application
// @Tags        app-media
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Application ID"
// @Router      /applications/applications/{id}/media [get]
func (h *ListHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	svc := resolveService(reqCtx)
	if svc == nil {
		return
	}
	appID := appIDFromPath(reqCtx)
	if appID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "missing application id")
		return
	}
	var parentID *string
	if p := queryParam(reqCtx, "parent_id"); p != "" {
		parentID = &p
	}
	listing, err := svc.Store.ListChildren(appID, parentID)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, listing)
}
