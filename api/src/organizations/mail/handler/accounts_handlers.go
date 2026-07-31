package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/a-digi/coco-iam/src/organizations/mail/entity"
	orgmail_persistent "github.com/a-digi/coco-iam/src/organizations/mail/repository/persistent"
	orgmail_query "github.com/a-digi/coco-iam/src/organizations/mail/repository/query"
	iam_mail "github.com/a-digi/coco-notification"
	mailsmtp "github.com/a-digi/coco-notification/mailer"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

func toAccountResponse(a orgmail_query.OrgMailAccount) entity.OrgMailAccountResponse {
	return entity.OrgMailAccountResponse{
		ID: a.ID, Name: a.Name, Host: a.Host, Port: a.Port, Username: a.Username,
		FromName: a.FromName, FromEmail: a.FromEmail, UseTLS: a.UseTLS, IsActive: a.IsActive,
		CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt,
	}
}

// OrgMailAccountsListHandler serves GET /organizations/{id}/mail/accounts.
type OrgMailAccountsListHandler struct{}

// @Summary     List an organization's SMTP accounts
// @Tags        org-mail
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Organization ID"
// @Success     200 {object} entity.OrgMailAccountListSuccess
// @Failure     400,401,403,404,500 {object} response.ErrorBody
// @Router      /organizations/{id}/mail/accounts [get]
func (h *OrgMailAccountsListHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	db, ok := resolveOrgDB(reqCtx)
	if !ok {
		return
	}
	list, err := orgmail_query.NewOrgMailAccountsQueryRepo(db).List()
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]entity.OrgMailAccountResponse, 0, len(list))
	for _, a := range list {
		out = append(out, toAccountResponse(a))
	}
	response.SuccessResponse(w, http.StatusOK, out)
}

// OrgMailAccountsGetHandler serves GET /organizations/{id}/mail/accounts/{accountId}.
type OrgMailAccountsGetHandler struct{}

// @Summary     Get an organization SMTP account
// @Tags        org-mail
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Organization ID"
// @Param       accountId path string true "Account ID"
// @Success     200 {object} entity.OrgMailAccountSuccess
// @Failure     400,401,403,404,500 {object} response.ErrorBody
// @Router      /organizations/{id}/mail/accounts/{accountId} [get]
func (h *OrgMailAccountsGetHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	db, ok := resolveOrgDB(reqCtx)
	if !ok {
		return
	}
	accountID, ok := nestedID(reqCtx, "accountId")
	if !ok {
		return
	}
	acc, err := orgmail_query.NewOrgMailAccountsQueryRepo(db).Get(accountID)
	if err != nil {
		if errors.Is(err, orgmail_query.ErrNotFound) {
			response.ErrorResponse(w, http.StatusNotFound, "account not found")
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, toAccountResponse(*acc))
}

// OrgMailAccountsCreateHandler serves POST /organizations/{id}/mail/accounts.
type OrgMailAccountsCreateHandler struct{}

// @Summary     Create an organization SMTP account
// @Tags        org-mail
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Organization ID"
// @Param       body body entity.OrgMailAccountCreateRequest true "Account"
// @Success     201 {object} entity.OrgMailAccountSuccess
// @Failure     400,401,403,404,409,500 {object} response.ErrorBody
// @Router      /organizations/{id}/mail/accounts [post]
func (h *OrgMailAccountsCreateHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	db, ok := resolveOrgDB(reqCtx)
	if !ok {
		return
	}

	var req entity.OrgMailAccountCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	defer r.Body.Close()

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || req.Host == "" || req.FromEmail == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "name, host, and from_email are required")
		return
	}

	id, err := orgmail_persistent.NewOrgMailAccountsPersistentRepo(db).Create(orgmail_persistent.OrgMailAccountWrite{
		Name: req.Name, Host: req.Host, Port: req.Port, Username: req.Username, Password: req.Password,
		FromName: req.FromName, FromEmail: req.FromEmail, UseTLS: req.UseTLS, IsActive: req.IsActive,
	})
	if err != nil {
		if errors.Is(err, orgmail_persistent.ErrDuplicateName) {
			response.ErrorResponse(w, http.StatusConflict, "an account with this name already exists")
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	acc, err := orgmail_query.NewOrgMailAccountsQueryRepo(db).Get(id)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusCreated, toAccountResponse(*acc))
}

// OrgMailAccountsUpdateHandler serves PATCH /organizations/{id}/mail/accounts/{accountId}.
type OrgMailAccountsUpdateHandler struct{}

// @Summary     Update an organization SMTP account
// @Tags        org-mail
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Organization ID"
// @Param       accountId path string true "Account ID"
// @Param       body body entity.OrgMailAccountUpdateRequest true "Patch"
// @Success     200 {object} entity.OrgMailAccountSuccess
// @Failure     400,401,403,404,500 {object} response.ErrorBody
// @Router      /organizations/{id}/mail/accounts/{accountId} [patch]
func (h *OrgMailAccountsUpdateHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	db, ok := resolveOrgDB(reqCtx)
	if !ok {
		return
	}
	accountID, ok := nestedID(reqCtx, "accountId")
	if !ok {
		return
	}

	var patch entity.OrgMailAccountUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	defer r.Body.Close()

	queryRepo := orgmail_query.NewOrgMailAccountsQueryRepo(db)
	existing, err := queryRepo.Get(accountID)
	if err != nil {
		if errors.Is(err, orgmail_query.ErrNotFound) {
			response.ErrorResponse(w, http.StatusNotFound, "account not found")
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	merged := orgmail_persistent.OrgMailAccountWrite{
		ID: existing.ID, Host: existing.Host, Port: existing.Port, Username: existing.Username,
		Password: existing.Password, FromName: existing.FromName, FromEmail: existing.FromEmail, UseTLS: existing.UseTLS,
	}
	if patch.Host != nil {
		merged.Host = *patch.Host
	}
	if patch.Port != nil {
		merged.Port = *patch.Port
	}
	if patch.Username != nil {
		merged.Username = *patch.Username
	}
	if patch.Password != nil && *patch.Password != "" {
		merged.Password = *patch.Password
	}
	if patch.FromName != nil {
		merged.FromName = *patch.FromName
	}
	if patch.FromEmail != nil {
		merged.FromEmail = *patch.FromEmail
	}
	if patch.UseTLS != nil {
		merged.UseTLS = *patch.UseTLS
	}

	if err := orgmail_persistent.NewOrgMailAccountsPersistentRepo(db).Update(merged); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	after, err := queryRepo.Get(accountID)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, toAccountResponse(*after))
}

// OrgMailAccountsDeleteHandler serves DELETE /organizations/{id}/mail/accounts/{accountId}.
type OrgMailAccountsDeleteHandler struct{}

// @Summary     Delete an organization SMTP account
// @Tags        org-mail
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Organization ID"
// @Param       accountId path string true "Account ID"
// @Success     200 {object} map[string]string
// @Failure     400,401,403,404,409,500 {object} response.ErrorBody
// @Router      /organizations/{id}/mail/accounts/{accountId} [delete]
func (h *OrgMailAccountsDeleteHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	db, ok := resolveOrgDB(reqCtx)
	if !ok {
		return
	}
	accountID, ok := nestedID(reqCtx, "accountId")
	if !ok {
		return
	}
	if err := orgmail_persistent.NewOrgMailAccountsPersistentRepo(db).Delete(accountID); err != nil {
		switch {
		case errors.Is(err, orgmail_persistent.ErrNotFound):
			response.ErrorResponse(w, http.StatusNotFound, "account not found")
		case errors.Is(err, orgmail_persistent.ErrActiveAccount):
			response.ErrorResponse(w, http.StatusConflict, "cannot delete the active account — activate another first")
		default:
			response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	response.SuccessResponse(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// OrgMailAccountsActivateHandler serves POST /organizations/{id}/mail/accounts/{accountId}/activate.
type OrgMailAccountsActivateHandler struct{}

// @Summary     Activate an organization SMTP account
// @Description Makes this the org's active account; every other org account is demoted. An org
// @Description with no active account falls back to the global active account at send time.
// @Tags        org-mail
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Organization ID"
// @Param       accountId path string true "Account ID"
// @Success     200 {object} entity.OrgMailAccountSuccess
// @Failure     400,401,403,404,500 {object} response.ErrorBody
// @Router      /organizations/{id}/mail/accounts/{accountId}/activate [post]
func (h *OrgMailAccountsActivateHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	db, ok := resolveOrgDB(reqCtx)
	if !ok {
		return
	}
	accountID, ok := nestedID(reqCtx, "accountId")
	if !ok {
		return
	}
	if err := orgmail_persistent.NewOrgMailAccountsPersistentRepo(db).Activate(accountID); err != nil {
		if errors.Is(err, orgmail_persistent.ErrNotFound) {
			response.ErrorResponse(w, http.StatusNotFound, "account not found")
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	acc, err := orgmail_query.NewOrgMailAccountsQueryRepo(db).Get(accountID)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, toAccountResponse(*acc))
}

// OrgMailAccountsTestHandler serves POST /organizations/{id}/mail/accounts/{accountId}/test.
// Mirrors AdminMailAccountsTestHandler exactly: a synchronous, one-off
// send bypassing the queue so an admin can verify credentials directly.
type OrgMailAccountsTestHandler struct{}

type orgAccountTestRequest struct {
	To   string `json:"to"`
	Name string `json:"name"`
}

// @Summary     Send a test email using an organization SMTP account
// @Tags        org-mail
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Organization ID"
// @Param       accountId path string true "Account ID"
// @Success     200 {object} map[string]string
// @Failure     400,401,403,404,500,502 {object} response.ErrorBody
// @Router      /organizations/{id}/mail/accounts/{accountId}/test [post]
func (h *OrgMailAccountsTestHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	db, ok := resolveOrgDB(reqCtx)
	if !ok {
		return
	}
	accountID, ok := nestedID(reqCtx, "accountId")
	if !ok {
		return
	}

	var req orgAccountTestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	defer r.Body.Close()
	req.To = strings.TrimSpace(req.To)
	if req.To == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "`to` is required")
		return
	}
	if req.Name == "" {
		req.Name = "admin"
	}

	acc, err := orgmail_query.NewOrgMailAccountsQueryRepo(db).Get(accountID)
	if err != nil {
		if errors.Is(err, orgmail_query.ErrNotFound) {
			response.ErrorResponse(w, http.StatusNotFound, "account not found")
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	subject, textBody, htmlBody := buildOrgTestBody(req.Name)
	cfg := mailsmtp.Config{
		Host: acc.Host, Port: acc.Port, Username: acc.Username, Password: acc.Password, UseTLS: acc.UseTLS,
		From: iam_mail.Address{Name: acc.FromName, Email: acc.FromEmail},
	}
	log := reqCtx.GetDI().GetLogger()
	mailer := mailsmtp.New(cfg, log)

	msg := iam_mail.Message{
		From: cfg.From, To: []iam_mail.Address{{Email: req.To}}, Subject: subject, TextBody: textBody, HTMLBody: htmlBody,
	}
	sendCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := mailer.Send(sendCtx, msg); err != nil {
		response.ErrorResponse(w, http.StatusBadGateway, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, map[string]string{
		"status": "sent", "to": req.To, "account": acc.Name, "timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

func buildOrgTestBody(name string) (subject, text, html string) {
	subject = "coco-iam — organization SMTP account test"
	text = "Hello " + name + ",\n\nThis is a test email from this organization mail account.\n\nIf you received this, the account is configured correctly.\n\n— coco-iam"
	html = `<!DOCTYPE html><html><body style="font-family: -apple-system, Segoe UI, Roboto, sans-serif; color: #111;">` +
		`<h2 style="color:#4f46e5;">coco-iam</h2>` +
		`<p>Hello <strong>` + name + `</strong>,</p>` +
		`<p>This is a test email from this organization mail account.</p>` +
		`<p style="color:#6b7280;font-size:12px;">If you received this, the account is configured correctly.</p>` +
		`</body></html>`
	return
}
