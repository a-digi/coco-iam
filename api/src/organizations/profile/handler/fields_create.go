package handler

import (
	"net/http"
	"strings"

	"github.com/a-digi/coco-iam/src/organizations/profile/entity"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// OrgProfileFieldsCreateHandler — POST /api/v1/organizations/{orgId}/profile-fields
type OrgProfileFieldsCreateHandler struct{}

type createFieldRequest struct {
	Name        string   `json:"name"`
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

// @Summary     Create organization profile field
// @Tags        org-profile-fields
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Organization ID"
// @Router      /organizations/organizations/{id}/profile-fields [post]
func (h *OrgProfileFieldsCreateHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	_, repo, err := repositoryFromRequest(reqCtx)
	if err != nil {
		writeErr(w, err)
		return
	}

	var body createFieldRequest
	if err := decodeJSONBody(reqCtx, &body); err != nil {
		writeErr(w, err)
		return
	}

	name := strings.TrimSpace(body.Name)
	label := strings.TrimSpace(body.Label)
	if name == "" || label == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "name and label are required")
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

	field := &entity.ProfileField{
		Name:        name,
		Label:       label,
		Description: body.Description,
		DataType:    body.DataType,
		IsRequired:  body.IsRequired,
		MinValue:    body.MinValue,
		MaxValue:    body.MaxValue,
		Options:     body.Options,
		Regex:       body.Regex,
		AcceptMime:  strings.TrimSpace(body.AcceptMime),
		MaxBytes:    body.MaxBytes,
	}
	if err := repo.CreateField(field); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to create field: "+err.Error())
		return
	}

	response.SuccessResponse(w, http.StatusCreated, field)
}
