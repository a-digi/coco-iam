package accounts

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/a-digi/coco-lift/resource/uri"
	iam_mail "github.com/a-digi/coco-notification"
	mailaccounts "github.com/a-digi/coco-notification/mailer"
	mailsmtp "github.com/a-digi/coco-notification/mailer"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// AdminMailAccountsTestHandler serves
// POST /api/v1/admin/mail/accounts/{id}/test. Sends a test email
// synchronously using the specific account's config — bypasses the queue
// so admins can verify credentials without creating audit rows or
// retries. Returns 200 on success, 502 on SMTP failure with the raw
// error message.
type AdminMailAccountsTestHandler struct{}

type testRequest struct {
	To   string `json:"to"`
	Name string `json:"name"`
}

// @Summary     Test a mail account
// @Tags        admin-mail
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Account ID"
// @Param       body body interface{} true "Request body"
// @Success     200 {object} interface{}
// @Failure     400,401,403,500 {object} map[string]interface{}
// @Router      /admin/mail/accounts/{id}/test [post]
func (h *AdminMailAccountsTestHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()
	key, value := uri.ExtractKeyAndValueFromURI(r.URL.Path)
	if key != "id" || value == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "account id is required")
		return
	}

	var req testRequest
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

	store := resolveStore(reqCtx)
	if store == nil {
		return
	}
	acc, err := store.Get(value)
	if err != nil {
		if errors.Is(err, mailaccounts.ErrNotFound) {
			response.ErrorResponse(w, http.StatusNotFound, "account not found")
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Inline test body — deliberately independent of the template
	// catalogue so the diagnostic works even when renderers are
	// misconfigured.
	subject, textBody, htmlBody := buildTestBody(req.Name)

	cfg := mailsmtp.Config{
		Host:     acc.Host,
		Port:     acc.Port,
		Username: acc.Username,
		Password: acc.Password,
		UseTLS:   acc.UseTLS,
		From:     iam_mail.Address{Name: acc.FromName, Email: acc.FromEmail},
	}
	log := reqCtx.GetDI().GetLogger()
	mailer := mailsmtp.New(cfg, log)

	msg := iam_mail.Message{
		From:     cfg.From,
		To:       []iam_mail.Address{{Email: req.To}},
		Subject:  subject,
		TextBody: textBody,
		HTMLBody: htmlBody,
	}

	sendCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := mailer.Send(sendCtx, msg); err != nil {
		response.ErrorResponse(w, http.StatusBadGateway, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, map[string]string{
		"status":    "sent",
		"to":        req.To,
		"account":   acc.Name,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// buildTestBody avoids a dependency on the Renderer (which isn't exposed
// via DI) by emitting a minimal test body inline. Good enough for the
// "does SMTP work?" diagnostic this handler serves.
func buildTestBody(name string) (subject, text, html string) {
	subject = "coco-iam — SMTP account test"
	text = "Hello " + name + ",\n\nThis is a test email from the coco-iam mail engine.\n\nIf you received this, your SMTP account is configured correctly.\n\n— coco-iam"
	html = `<!DOCTYPE html><html><body style="font-family: -apple-system, Segoe UI, Roboto, sans-serif; color: #111;">` +
		`<h2 style="color:#4f46e5;">coco-iam</h2>` +
		`<p>Hello <strong>` + name + `</strong>,</p>` +
		`<p>This is a test email from the coco-iam mail engine.</p>` +
		`<p style="color:#6b7280;font-size:12px;">If you received this, your SMTP account is configured correctly.</p>` +
		`</body></html>`
	return
}
