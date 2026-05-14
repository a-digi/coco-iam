package handler

import (
	"net/http"

	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// OrgProfileFieldsListHandler — GET /api/v1/organizations/{orgId}/profile-fields
// Returns all fields (active + soft-deleted) ordered by order_index.
type OrgProfileFieldsListHandler struct{}

func (h *OrgProfileFieldsListHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	_, repo, err := repositoryFromRequest(reqCtx)
	if err != nil {
		writeErr(w, err)
		return
	}
	fields, err := repo.ListFields(false)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, fields)
}
