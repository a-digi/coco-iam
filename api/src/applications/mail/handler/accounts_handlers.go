package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/a-digi/coco-iam/src/applications/mail/entity"
	appmail_persistent "github.com/a-digi/coco-iam/src/applications/mail/repository/persistent"
	appmail_query "github.com/a-digi/coco-iam/src/applications/mail/repository/query"
	iam_mail "github.com/a-digi/coco-notification"
	mailsmtp "github.com/a-digi/coco-notification/mailer"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

func toAccountResponse(a appmail_query.AppMailAccount) entity.AppMailAccountResponse {
	return entity.AppMailAccountResponse{
		ID: a.ID, Name: a.Name, Host: a.Host, Port: a.Port, Username: a.Username,
		FromName: a.FromName, FromEmail: a.FromEmail, UseTLS: a.UseTLS, IsActive: a.IsActive,
		CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt,
	}
}

// AppMailAccountsListHandler serves GET /applications/{id}/mail/accounts.
type AppMailAccountsListHandler struct{}

// @Summary     List an application's SMTP accounts
// @Tags        app-mail
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Application ID"
// @Success     200 {object} entity.AppMailAccountListSuccess
// @Failure     400,401,403,404,500 {object} response.ErrorBody
// @Router      /applications/{id}/mail/accounts [get]
func (h *AppMailAccountsListHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	db, appID, ok := resolveAppDB(reqCtx)
	if !ok {
		return
	}
	list, err := appmail_query.NewAppMailAccountsQueryRepo(db, appID).List()
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]entity.AppMailAccountResponse, 0, len(list))
	for _, a := range list {
		out = append(out, toAccountResponse(a))
	}
	response.SuccessResponse(w, http.StatusOK, out)
}

// AppMailAccountsGetHandler serves GET /applications/{id}/mail/accounts/{accountId}.
type AppMailAccountsGetHandler struct{}

// @Summary     Get an application SMTP account
// @Tags        app-mail
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Application ID"
// @Param       accountId path string true "Account ID"
// @Success     200 {object} entity.AppMailAccountSuccess
// @Failure     400,401,403,404,500 {object} response.ErrorBody
// @Router      /applications/{id}/mail/accounts/{accountId} [get]
func (h *AppMailAccountsGetHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	db, appID, ok := resolveAppDB(reqCtx)
	if !ok {
		return
	}
	accountID, ok := nestedID(reqCtx, "accountId")
	if !ok {
		return
	}
	acc, err := appmail_query.NewAppMailAccountsQueryRepo(db, appID).Get(accountID)
	if err != nil {
		if errors.Is(err, appmail_query.ErrNotFound) {
			response.ErrorResponse(w, http.StatusNotFound, "account not found")
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, toAccountResponse(*acc))
}

// AppMailAccountsCreateHandler serves POST /applications/{id}/mail/accounts.
type AppMailAccountsCreateHandler struct{}

// @Summary     Create an application SMTP account
// @Tags        app-mail
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Application ID"
// @Param       body body entity.AppMailAccountCreateRequest true "Account"
// @Success     201 {object} entity.AppMailAccountSuccess
// @Failure     400,401,403,404,409,500 {object} response.ErrorBody
// @Router      /applications/{id}/mail/accounts [post]
func (h *AppMailAccountsCreateHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	db, appID, ok := resolveAppDB(reqCtx)
	if !ok {
		return
	}

	var req entity.AppMailAccountCreateRequest
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

	id, err := appmail_persistent.NewAppMailAccountsPersistentRepo(db, appID).Create(appmail_persistent.AppMailAccountWrite{
		Name: req.Name, Host: req.Host, Port: req.Port, Username: req.Username, Password: req.Password,
		FromName: req.FromName, FromEmail: req.FromEmail, UseTLS: req.UseTLS, IsActive: req.IsActive,
	})
	if err != nil {
		if errors.Is(err, appmail_persistent.ErrDuplicateName) {
			response.ErrorResponse(w, http.StatusConflict, "an account with this name already exists")
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	acc, err := appmail_query.NewAppMailAccountsQueryRepo(db, appID).Get(id)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusCreated, toAccountResponse(*acc))
}

// AppMailAccountsUpdateHandler serves PATCH /applications/{id}/mail/accounts/{accountId}.
type AppMailAccountsUpdateHandler struct{}

// @Summary     Update an application SMTP account
// @Tags        app-mail
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Application ID"
// @Param       accountId path string true "Account ID"
// @Param       body body entity.AppMailAccountUpdateRequest true "Patch"
// @Success     200 {object} entity.AppMailAccountSuccess
// @Failure     400,401,403,404,500 {object} response.ErrorBody
// @Router      /applications/{id}/mail/accounts/{accountId} [patch]
func (h *AppMailAccountsUpdateHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	db, appID, ok := resolveAppDB(reqCtx)
	if !ok {
		return
	}
	accountID, ok := nestedID(reqCtx, "accountId")
	if !ok {
		return
	}

	var patch entity.AppMailAccountUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	defer r.Body.Close()

	queryRepo := appmail_query.NewAppMailAccountsQueryRepo(db, appID)
	existing, err := queryRepo.Get(accountID)
	if err != nil {
		if errors.Is(err, appmail_query.ErrNotFound) {
			response.ErrorResponse(w, http.StatusNotFound, "account not found")
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	merged := appmail_persistent.AppMailAccountWrite{
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

	if err := appmail_persistent.NewAppMailAccountsPersistentRepo(db, appID).Update(merged); err != nil {
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

// AppMailAccountsDeleteHandler serves DELETE /applications/{id}/mail/accounts/{accountId}.
type AppMailAccountsDeleteHandler struct{}

// @Summary     Delete an application SMTP account
// @Tags        app-mail
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Application ID"
// @Param       accountId path string true "Account ID"
// @Success     200 {object} map[string]string
// @Failure     400,401,403,404,409,500 {object} response.ErrorBody
// @Router      /applications/{id}/mail/accounts/{accountId} [delete]
func (h *AppMailAccountsDeleteHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	db, appID, ok := resolveAppDB(reqCtx)
	if !ok {
		return
	}
	accountID, ok := nestedID(reqCtx, "accountId")
	if !ok {
		return
	}
	if err := appmail_persistent.NewAppMailAccountsPersistentRepo(db, appID).Delete(accountID); err != nil {
		switch {
		case errors.Is(err, appmail_persistent.ErrNotFound):
			response.ErrorResponse(w, http.StatusNotFound, "account not found")
		case errors.Is(err, appmail_persistent.ErrActiveAccount):
			response.ErrorResponse(w, http.StatusConflict, "cannot delete the active account — activate another first")
		default:
			response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	response.SuccessResponse(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// AppMailAccountsActivateHandler serves POST /applications/{id}/mail/accounts/{accountId}/activate.
type AppMailAccountsActivateHandler struct{}

// @Summary     Activate an application SMTP account
// @Description Makes this the application's active account; every other application account
// @Description is demoted. An application with no active account falls back to the
// @Description organization's, then the global, active account at send time.
// @Tags        app-mail
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Application ID"
// @Param       accountId path string true "Account ID"
// @Success     200 {object} entity.AppMailAccountSuccess
// @Failure     400,401,403,404,500 {object} response.ErrorBody
// @Router      /applications/{id}/mail/accounts/{accountId}/activate [post]
func (h *AppMailAccountsActivateHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	db, appID, ok := resolveAppDB(reqCtx)
	if !ok {
		return
	}
	accountID, ok := nestedID(reqCtx, "accountId")
	if !ok {
		return
	}
	if err := appmail_persistent.NewAppMailAccountsPersistentRepo(db, appID).Activate(accountID); err != nil {
		if errors.Is(err, appmail_persistent.ErrNotFound) {
			response.ErrorResponse(w, http.StatusNotFound, "account not found")
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	acc, err := appmail_query.NewAppMailAccountsQueryRepo(db, appID).Get(accountID)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, toAccountResponse(*acc))
}

// AppMailAccountsTestHandler serves POST /applications/{id}/mail/accounts/{accountId}/test.
// Mirrors OrgMailAccountsTestHandler exactly: a synchronous, one-off
// send bypassing the queue so an admin can verify credentials directly.
type AppMailAccountsTestHandler struct{}

type appAccountTestRequest struct {
	To   string `json:"to"`
	Name string `json:"name"`
}

// @Summary     Send a test email using an application SMTP account
// @Tags        app-mail
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Application ID"
// @Param       accountId path string true "Account ID"
// @Success     200 {object} map[string]string
// @Failure     400,401,403,404,500,502 {object} response.ErrorBody
// @Router      /applications/{id}/mail/accounts/{accountId}/test [post]
func (h *AppMailAccountsTestHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	db, appID, ok := resolveAppDB(reqCtx)
	if !ok {
		return
	}
	accountID, ok := nestedID(reqCtx, "accountId")
	if !ok {
		return
	}

	var req appAccountTestRequest
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

	acc, err := appmail_query.NewAppMailAccountsQueryRepo(db, appID).Get(accountID)
	if err != nil {
		if errors.Is(err, appmail_query.ErrNotFound) {
			response.ErrorResponse(w, http.StatusNotFound, "account not found")
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	subject, textBody, htmlBody := buildAppTestBody(req.Name)
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

func buildAppTestBody(name string) (subject, text, html string) {
	subject = "coco-iam — application SMTP account test"
	text = "Hello " + name + ",\n\nThis is a test email from this application mail account.\n\nIf you received this, the account is configured correctly.\n\n— coco-iam"
	html = `<!DOCTYPE html><html><body style="font-family: -apple-system, Segoe UI, Roboto, sans-serif; color: #111;">` +
		`<h2 style="color:#4f46e5;">coco-iam</h2>` +
		`<p>Hello <strong>` + name + `</strong>,</p>` +
		`<p>This is a test email from this application mail account.</p>` +
		`<p style="color:#6b7280;font-size:12px;">If you received this, the account is configured correctly.</p>` +
		`</body></html>`
	return
}
