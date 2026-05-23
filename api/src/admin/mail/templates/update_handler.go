package templates

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/a-digi/coco-lift/resource/uri"
	"github.com/a-digi/coco-iam/src/mail/template"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// AdminMailTemplatesUpdateHandler serves PATCH /api/v1/admin/mail/templates/{id}.
// Name is intentionally not patchable — renderers address templates by name.
type AdminMailTemplatesUpdateHandler struct{}

type updateRequest struct {
	Description *string `json:"description,omitempty"`
	Subject     *string `json:"subject,omitempty"`
	TextBody    *string `json:"text_body,omitempty"`
	HTMLBody    *string `json:"html_body,omitempty"`
	IsActive    *bool   `json:"is_active,omitempty"`
	// Name is accepted but ignored with a 400 if set to a different value —
	// keeps API clients from silently thinking a rename worked.
	Name *string `json:"name,omitempty"`
}

// @Summary     Update a mail template
// @Tags        admin-mail-templates
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Template ID"
// @Param       body body interface{} true "Request body"
// @Success     200 {object} interface{}
// @Failure     400,401,403,500 {object} map[string]interface{}
// @Router      /admin/mail/templates/{id} [patch]
func (h *AdminMailTemplatesUpdateHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	key, value := uri.ExtractKeyAndValueFromURI(r.URL.Path)
	if key != "id" || value == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "template id is required")
		return
	}

	var req updateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	defer r.Body.Close()

	repo := resolveRepo(reqCtx)
	if repo == nil {
		return
	}

	if req.Name != nil {
		existing, err := repo.Get(value)
		if err != nil {
			if errors.Is(err, template.ErrNotFound) {
				response.ErrorResponse(w, http.StatusNotFound, "template not found")
				return
			}
			response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		if *req.Name != existing.Name {
			response.ErrorResponse(w, http.StatusBadRequest, "template name is immutable")
			return
		}
	}

	patch := template.Patch{
		Description: req.Description,
		Subject:     req.Subject,
		TextBody:    req.TextBody,
		HTMLBody:    req.HTMLBody,
		IsActive:    req.IsActive,
	}
	updated, err := repo.Update(value, patch)
	if err != nil {
		if errors.Is(err, template.ErrNotFound) {
			response.ErrorResponse(w, http.StatusNotFound, "template not found")
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, updated)
}
