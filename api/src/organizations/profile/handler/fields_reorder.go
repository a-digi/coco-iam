package handler

import (
	"net/http"

	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// OrgProfileFieldsReorderHandler — POST /api/v1/organizations/{orgId}/profile-fields/reorder
// Body: { "ordered_ids": ["id1", "id2", ...] }
// Sets each field's order_index to its 1-based index in the list.
type OrgProfileFieldsReorderHandler struct{}

type reorderRequest struct {
	OrderedIDs []string `json:"ordered_ids"`
}

func (h *OrgProfileFieldsReorderHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()

	_, repo, err := repositoryFromRequest(reqCtx)
	if err != nil {
		writeErr(w, err)
		return
	}

	var body reorderRequest
	if err := decodeJSONBody(reqCtx, &body); err != nil {
		writeErr(w, err)
		return
	}
	if len(body.OrderedIDs) == 0 {
		response.ErrorResponse(w, http.StatusBadRequest, "ordered_ids must not be empty")
		return
	}

	if err := repo.ReorderFields(body.OrderedIDs); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to reorder: "+err.Error())
		return
	}

	response.SuccessResponse(w, http.StatusOK, map[string]string{"status": "reordered"})
}
