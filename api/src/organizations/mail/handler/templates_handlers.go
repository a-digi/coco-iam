package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/a-digi/coco-iam/src/organizations/mail/entity"
	orgmail_persistent "github.com/a-digi/coco-iam/src/organizations/mail/repository/persistent"
	orgmail_query "github.com/a-digi/coco-iam/src/organizations/mail/repository/query"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

func toTemplateResponse(t orgmail_query.OrgMailTemplate) entity.OrgMailTemplateResponse {
	return entity.OrgMailTemplateResponse{
		ID: t.ID, Name: t.Name, Description: t.Description, Subject: t.Subject,
		TextBody: t.TextBody, HTMLBody: t.HTMLBody, IsActive: t.IsActive,
		CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt,
	}
}

// OrgMailTemplatesListHandler serves GET /organizations/{id}/mail/templates.
type OrgMailTemplatesListHandler struct{}

// @Summary     List an organization's email templates
// @Tags        org-mail
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Organization ID"
// @Param       name query string false "Filter by name substring"
// @Param       limit query int false "Page size (default 50, max 500)"
// @Param       offset query int false "Page offset"
// @Success     200 {object} entity.OrgMailTemplateListSuccess
// @Failure     400,401,403,404,500 {object} response.ErrorBody
// @Router      /organizations/{id}/mail/templates [get]
func (h *OrgMailTemplatesListHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()
	db, ok := resolveOrgDB(reqCtx)
	if !ok {
		return
	}

	q := r.URL.Query()
	filter := orgmail_query.OrgMailTemplateListFilter{NameLike: q.Get("name")}
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			filter.Limit = n
		}
	}
	if v := q.Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			filter.Offset = n
		}
	}

	list, total, err := orgmail_query.NewOrgMailTemplatesQueryRepo(db).List(filter)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]entity.OrgMailTemplateResponse, 0, len(list))
	for _, t := range list {
		out = append(out, toTemplateResponse(t))
	}
	response.SuccessResponse(w, http.StatusOK, entity.OrgMailTemplateListResponse{Items: out, Total: total})
}

// OrgMailTemplatesGetHandler serves GET /organizations/{id}/mail/templates/{templateId}.
type OrgMailTemplatesGetHandler struct{}

// @Summary     Get an organization email template
// @Tags        org-mail
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Organization ID"
// @Param       templateId path string true "Template ID"
// @Success     200 {object} entity.OrgMailTemplateSuccess
// @Failure     400,401,403,404,500 {object} response.ErrorBody
// @Router      /organizations/{id}/mail/templates/{templateId} [get]
func (h *OrgMailTemplatesGetHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	db, ok := resolveOrgDB(reqCtx)
	if !ok {
		return
	}
	templateID, ok := nestedID(reqCtx, "templateId")
	if !ok {
		return
	}
	t, err := orgmail_query.NewOrgMailTemplatesQueryRepo(db).Get(templateID)
	if err != nil {
		if errors.Is(err, orgmail_query.ErrTemplateNotFound) {
			response.ErrorResponse(w, http.StatusNotFound, "template not found")
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, toTemplateResponse(*t))
}

// OrgMailTemplatesCreateHandler serves POST /organizations/{id}/mail/templates.
type OrgMailTemplatesCreateHandler struct{}

// @Summary     Create an organization email template
// @Description The name should match one of the system event keys (see the global
// @Description /admin/mail/settings/events catalog) for it to be bindable to an event
// @Description via PATCH /organizations/{id}/mail/settings.
// @Tags        org-mail
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Organization ID"
// @Param       body body entity.OrgMailTemplateCreateRequest true "Template"
// @Success     201 {object} entity.OrgMailTemplateSuccess
// @Failure     400,401,403,404,409,500 {object} response.ErrorBody
// @Router      /organizations/{id}/mail/templates [post]
func (h *OrgMailTemplatesCreateHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	db, ok := resolveOrgDB(reqCtx)
	if !ok {
		return
	}

	var req entity.OrgMailTemplateCreateRequest
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

	id, err := orgmail_persistent.NewOrgMailTemplatesPersistentRepo(db).Create(orgmail_persistent.OrgMailTemplateWrite{
		Name: req.Name, Description: req.Description, Subject: req.Subject,
		TextBody: req.TextBody, HTMLBody: req.HTMLBody, IsActive: req.IsActive,
	})
	if err != nil {
		if errors.Is(err, orgmail_persistent.ErrTemplateDuplicateName) {
			response.ErrorResponse(w, http.StatusConflict, "a template with this name already exists")
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	t, err := orgmail_query.NewOrgMailTemplatesQueryRepo(db).Get(id)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusCreated, toTemplateResponse(*t))
}

// OrgMailTemplatesUpdateHandler serves PATCH /organizations/{id}/mail/templates/{templateId}.
type OrgMailTemplatesUpdateHandler struct{}

// @Summary     Update an organization email template
// @Tags        org-mail
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Organization ID"
// @Param       templateId path string true "Template ID"
// @Param       body body entity.OrgMailTemplateUpdateRequest true "Patch"
// @Success     200 {object} entity.OrgMailTemplateSuccess
// @Failure     400,401,403,404,500 {object} response.ErrorBody
// @Router      /organizations/{id}/mail/templates/{templateId} [patch]
func (h *OrgMailTemplatesUpdateHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	db, ok := resolveOrgDB(reqCtx)
	if !ok {
		return
	}
	templateID, ok := nestedID(reqCtx, "templateId")
	if !ok {
		return
	}

	var patch entity.OrgMailTemplateUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	defer r.Body.Close()

	queryRepo := orgmail_query.NewOrgMailTemplatesQueryRepo(db)
	existing, err := queryRepo.Get(templateID)
	if err != nil {
		if errors.Is(err, orgmail_query.ErrTemplateNotFound) {
			response.ErrorResponse(w, http.StatusNotFound, "template not found")
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	merged := orgmail_persistent.OrgMailTemplateWrite{
		ID: existing.ID, Description: existing.Description, Subject: existing.Subject,
		TextBody: existing.TextBody, HTMLBody: existing.HTMLBody, IsActive: existing.IsActive,
	}
	if patch.Description != nil {
		merged.Description = *patch.Description
	}
	if patch.Subject != nil {
		merged.Subject = *patch.Subject
	}
	if patch.TextBody != nil {
		merged.TextBody = *patch.TextBody
	}
	if patch.HTMLBody != nil {
		merged.HTMLBody = *patch.HTMLBody
	}
	if patch.IsActive != nil {
		merged.IsActive = *patch.IsActive
	}

	if err := orgmail_persistent.NewOrgMailTemplatesPersistentRepo(db).Update(merged); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	after, err := queryRepo.Get(templateID)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, toTemplateResponse(*after))
}

// OrgMailTemplatesDeleteHandler serves DELETE /organizations/{id}/mail/templates/{templateId}.
type OrgMailTemplatesDeleteHandler struct{}

// @Summary     Delete an organization email template
// @Tags        org-mail
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Organization ID"
// @Param       templateId path string true "Template ID"
// @Success     200 {object} map[string]string
// @Failure     400,401,403,404,500 {object} response.ErrorBody
// @Router      /organizations/{id}/mail/templates/{templateId} [delete]
func (h *OrgMailTemplatesDeleteHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	db, ok := resolveOrgDB(reqCtx)
	if !ok {
		return
	}
	templateID, ok := nestedID(reqCtx, "templateId")
	if !ok {
		return
	}
	if err := orgmail_persistent.NewOrgMailTemplatesPersistentRepo(db).Delete(templateID); err != nil {
		if errors.Is(err, orgmail_persistent.ErrTemplateNotFound) {
			response.ErrorResponse(w, http.StatusNotFound, "template not found")
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, map[string]string{"status": "deleted"})
}
