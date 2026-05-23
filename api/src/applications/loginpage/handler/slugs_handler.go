package handler

import (
	"errors"
	"net/http"

	"github.com/a-digi/coco-iam/src/applications/loginpage"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// SlugsResponse is the body of GET /api/v1/applications/{id}/slugs.
// It carries the admin-chosen identifier trio that admin UIs need to
// build public slug-based URLs (media, login page, etc.) without
// exposing UUIDs.
type SlugsResponse struct {
	OrganizationID string `json:"organization_id"`
	WorkspaceID    string `json:"workspace_id"`
	ClientID       string `json:"client_id"`
}

// AppSlugsHandler serves GET /api/v1/applications/{id}/slugs.
// Returns the slug trio for the application row identified by the
// UUID in the path. Scope: applications:read (declared in YAML).
type AppSlugsHandler struct{}

// @Summary     Get application slugs
// @Tags        applications
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Application ID"
// @Router      /applications/applications/{id}/slugs [get]
func (h *AppSlugsHandler) ServeHTTP(reqCtx request.RequestContext) {
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
	slugs, err := svc.Store.LookupSlugsByAppID(appID)
	if err != nil {
		if errors.Is(err, loginpage.ErrNotFound) {
			response.ErrorResponse(w, http.StatusNotFound, "application not found")
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, SlugsResponse{
		OrganizationID: slugs.OrganizationSlug,
		WorkspaceID:    slugs.WorkspaceSlug,
		ClientID:       slugs.ClientID,
	})
}
