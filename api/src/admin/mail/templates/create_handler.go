package templates

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/a-digi/coco-iam/src/mail/template"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// AdminMailTemplatesCreateHandler serves POST /api/v1/admin/mail/templates.
type AdminMailTemplatesCreateHandler struct{}

type createRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Subject     string `json:"subject"`
	TextBody    string `json:"text_body"`
	HTMLBody    string `json:"html_body"`
	IsActive    *bool  `json:"is_active,omitempty"`
}

// @Summary     Create a mail template
// @Tags        admin-mail-templates
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       body body interface{} true "Request body"
// @Success     200 {object} interface{}
// @Failure     400,401,403,500 {object} map[string]interface{}
// @Router      /admin/mail/templates [post]
func (h *AdminMailTemplatesCreateHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	defer r.Body.Close()

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "name is required")
		return
	}
	if !template.NameFormat.MatchString(req.Name) {
		response.ErrorResponse(w, http.StatusBadRequest,
			"name must match "+template.NameFormat.String()+" (lowercase letters, digits, _, -, starting with a letter)")
		return
	}
	if req.Subject == "" && req.TextBody == "" && req.HTMLBody == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "at least one of subject / text_body / html_body must be provided")
		return
	}
	if req.TextBody == "" && req.HTMLBody == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "at least one of text_body or html_body must be provided")
		return
	}

	repo := resolveRepo(reqCtx)
	if repo == nil {
		return
	}

	t := template.Template{
		Name:        req.Name,
		Description: req.Description,
		Subject:     req.Subject,
		TextBody:    req.TextBody,
		HTMLBody:    req.HTMLBody,
		IsActive:    true,
	}
	if req.IsActive != nil {
		t.IsActive = *req.IsActive
	}

	created, err := repo.Create(t)
	if err != nil {
		if errors.Is(err, template.ErrDuplicateName) {
			response.ErrorResponse(w, http.StatusConflict, "a template with that name already exists")
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusCreated, created)
}
