package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/a-digi/coco-iam/src/applications/mail/entity"
	appmail_persistent "github.com/a-digi/coco-iam/src/applications/mail/repository/persistent"
	appmail_query "github.com/a-digi/coco-iam/src/applications/mail/repository/query"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

func toTemplateResponse(t appmail_query.AppMailTemplate) entity.AppMailTemplateResponse {
	return entity.AppMailTemplateResponse{
		ID: t.ID, Name: t.Name, Description: t.Description, Subject: t.Subject,
		TextBody: t.TextBody, HTMLBody: t.HTMLBody, IsActive: t.IsActive,
		CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt,
	}
}

// AppMailTemplatesListHandler serves GET /applications/{id}/mail/templates.
type AppMailTemplatesListHandler struct{}

// @Summary     List an application's email templates
// @Tags        app-mail
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Application ID"
// @Param       name query string false "Filter by name substring"
// @Param       limit query int false "Page size (default 50, max 500)"
// @Param       offset query int false "Page offset"
// @Success     200 {object} entity.AppMailTemplateListSuccess
// @Failure     400,401,403,404,500 {object} response.ErrorBody
// @Router      /applications/{id}/mail/templates [get]
func (h *AppMailTemplatesListHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()
	db, appID, ok := resolveAppDB(reqCtx)
	if !ok {
		return
	}

	q := r.URL.Query()
	filter := appmail_query.AppMailTemplateListFilter{NameLike: q.Get("name")}
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

	list, total, err := appmail_query.NewAppMailTemplatesQueryRepo(db, appID).List(filter)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]entity.AppMailTemplateResponse, 0, len(list))
	for _, t := range list {
		out = append(out, toTemplateResponse(t))
	}
	response.SuccessResponse(w, http.StatusOK, entity.AppMailTemplateListResponse{Items: out, Total: total})
}

// AppMailTemplatesGetHandler serves GET /applications/{id}/mail/templates/{templateId}.
type AppMailTemplatesGetHandler struct{}

// @Summary     Get an application email template
// @Tags        app-mail
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Application ID"
// @Param       templateId path string true "Template ID"
// @Success     200 {object} entity.AppMailTemplateSuccess
// @Failure     400,401,403,404,500 {object} response.ErrorBody
// @Router      /applications/{id}/mail/templates/{templateId} [get]
func (h *AppMailTemplatesGetHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	db, appID, ok := resolveAppDB(reqCtx)
	if !ok {
		return
	}
	templateID, ok := nestedID(reqCtx, "templateId")
	if !ok {
		return
	}
	t, err := appmail_query.NewAppMailTemplatesQueryRepo(db, appID).Get(templateID)
	if err != nil {
		if errors.Is(err, appmail_query.ErrTemplateNotFound) {
			response.ErrorResponse(w, http.StatusNotFound, "template not found")
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, toTemplateResponse(*t))
}

// AppMailTemplatesCreateHandler serves POST /applications/{id}/mail/templates.
type AppMailTemplatesCreateHandler struct{}

// @Summary     Create an application email template
// @Description The name should match one of the system event keys (see the global
// @Description /admin/mail/settings/events catalog) for it to be bindable to an event
// @Description via PATCH /applications/{id}/mail/settings.
// @Tags        app-mail
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Application ID"
// @Param       body body entity.AppMailTemplateCreateRequest true "Template"
// @Success     201 {object} entity.AppMailTemplateSuccess
// @Failure     400,401,403,404,409,500 {object} response.ErrorBody
// @Router      /applications/{id}/mail/templates [post]
func (h *AppMailTemplatesCreateHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	db, appID, ok := resolveAppDB(reqCtx)
	if !ok {
		return
	}

	var req entity.AppMailTemplateCreateRequest
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

	id, err := appmail_persistent.NewAppMailTemplatesPersistentRepo(db, appID).Create(appmail_persistent.AppMailTemplateWrite{
		Name: req.Name, Description: req.Description, Subject: req.Subject,
		TextBody: req.TextBody, HTMLBody: req.HTMLBody, IsActive: req.IsActive,
	})
	if err != nil {
		if errors.Is(err, appmail_persistent.ErrTemplateDuplicateName) {
			response.ErrorResponse(w, http.StatusConflict, "a template with this name already exists")
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	t, err := appmail_query.NewAppMailTemplatesQueryRepo(db, appID).Get(id)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusCreated, toTemplateResponse(*t))
}

// AppMailTemplatesUpdateHandler serves PATCH /applications/{id}/mail/templates/{templateId}.
type AppMailTemplatesUpdateHandler struct{}

// @Summary     Update an application email template
// @Tags        app-mail
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Application ID"
// @Param       templateId path string true "Template ID"
// @Param       body body entity.AppMailTemplateUpdateRequest true "Patch"
// @Success     200 {object} entity.AppMailTemplateSuccess
// @Failure     400,401,403,404,500 {object} response.ErrorBody
// @Router      /applications/{id}/mail/templates/{templateId} [patch]
func (h *AppMailTemplatesUpdateHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	db, appID, ok := resolveAppDB(reqCtx)
	if !ok {
		return
	}
	templateID, ok := nestedID(reqCtx, "templateId")
	if !ok {
		return
	}

	var patch entity.AppMailTemplateUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	defer r.Body.Close()

	queryRepo := appmail_query.NewAppMailTemplatesQueryRepo(db, appID)
	existing, err := queryRepo.Get(templateID)
	if err != nil {
		if errors.Is(err, appmail_query.ErrTemplateNotFound) {
			response.ErrorResponse(w, http.StatusNotFound, "template not found")
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	merged := appmail_persistent.AppMailTemplateWrite{
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

	if err := appmail_persistent.NewAppMailTemplatesPersistentRepo(db, appID).Update(merged); err != nil {
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

// AppMailTemplatesDeleteHandler serves DELETE /applications/{id}/mail/templates/{templateId}.
type AppMailTemplatesDeleteHandler struct{}

// @Summary     Delete an application email template
// @Tags        app-mail
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Application ID"
// @Param       templateId path string true "Template ID"
// @Success     200 {object} map[string]string
// @Failure     400,401,403,404,500 {object} response.ErrorBody
// @Router      /applications/{id}/mail/templates/{templateId} [delete]
func (h *AppMailTemplatesDeleteHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	db, appID, ok := resolveAppDB(reqCtx)
	if !ok {
		return
	}
	templateID, ok := nestedID(reqCtx, "templateId")
	if !ok {
		return
	}
	if err := appmail_persistent.NewAppMailTemplatesPersistentRepo(db, appID).Delete(templateID); err != nil {
		if errors.Is(err, appmail_persistent.ErrTemplateNotFound) {
			response.ErrorResponse(w, http.StatusNotFound, "template not found")
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, map[string]string{"status": "deleted"})
}
