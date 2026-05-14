package handler

import (
	"net/http"
	"strings"

	"github.com/a-digi/coco-iam/src/organizations/profile/entity"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// OrgProfileFieldsUpdateHandler — PATCH /api/v1/organizations/{orgId}/profile-fields/{fieldId}
// Edits label/description/type/required/min/max/options/regex.
// Name is NOT editable — changing the name would break stored profile_data keys.
type OrgProfileFieldsUpdateHandler struct{}

type updateFieldRequest struct {
	Label       string   `json:"label"`
	Description string   `json:"description"`
	DataType    string   `json:"data_type"`
	IsRequired  bool     `json:"is_required"`
	MinValue    *int     `json:"min_value"`
	MaxValue    *int     `json:"max_value"`
	Options     []string `json:"options"`
	Regex       string   `json:"regex"`
	AcceptMime  string   `json:"accept_mime"`
	MaxBytes    int      `json:"max_bytes"`
}

func (h *OrgProfileFieldsUpdateHandler) ServeHTTP(reqCtx request.RequestContext) {
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

	var body updateFieldRequest
	if err := decodeJSONBody(reqCtx, &body); err != nil {
		writeErr(w, err)
		return
	}
	label := strings.TrimSpace(body.Label)
	if label == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "label is required")
		return
	}
	if !entity.IsAllowedDataType(body.DataType) {
		response.ErrorResponse(w, http.StatusBadRequest, "unsupported data_type")
		return
	}
	if entity.RequiresOptions(body.DataType) && len(body.Options) == 0 {
		response.ErrorResponse(w, http.StatusBadRequest, "this field type requires at least one option")
		return
	}

	existing.Label = label
	existing.Description = body.Description
	existing.DataType = body.DataType
	existing.IsRequired = body.IsRequired
	existing.MinValue = body.MinValue
	existing.MaxValue = body.MaxValue
	existing.Options = body.Options
	existing.Regex = body.Regex
	existing.AcceptMime = strings.TrimSpace(body.AcceptMime)
	existing.MaxBytes = body.MaxBytes

	if err := repo.UpdateField(existing); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to update field: "+err.Error())
		return
	}

	response.SuccessResponse(w, http.StatusOK, existing)
}
