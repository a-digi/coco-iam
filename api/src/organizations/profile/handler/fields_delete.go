package handler

import (
	"net/http"

	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// OrgProfileFieldsDeleteHandler — DELETE /api/v1/organizations/{orgId}/profile-fields/{fieldId}
// Soft-delete only. Values stored on users remain but won't be rendered in forms.
type OrgProfileFieldsDeleteHandler struct{}

// @Summary     Delete organization profile field
// @Tags        org-profile-fields
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Organization ID"
// @Param       fieldId path string true "Field ID"
// @Router      /organizations/organizations/{id}/profile-fields/{fieldId} [delete]
func (h *OrgProfileFieldsDeleteHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	_, repo, err := repositoryFromRequest(reqCtx)
	if err != nil {
		writeErr(w, err)
		return
	}

	fieldID := extractToken(r.URL.Path, "fieldId")
	if fieldID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "field id is required")
		return
	}

	existing, err := repo.GetField(fieldID)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	if existing == nil {
		response.ErrorResponse(w, http.StatusNotFound, "field not found")
		return
	}

	if err := repo.SoftDeleteField(fieldID); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to delete field: "+err.Error())
		return
	}

	response.SuccessResponse(w, http.StatusOK, map[string]string{"id": fieldID, "status": "deleted"})
}
