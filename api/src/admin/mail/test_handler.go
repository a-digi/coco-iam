// Package admin holds HTTP handlers for mail administration. The single
// endpoint for now is a test-email trigger that helps verify SMTP wiring
// without running manual Go code.
package admin

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/a-digi/coco-iam/config/di"
	iam_notification "github.com/a-digi/coco-iam/src/notification"
	coconotification "github.com/a-digi/coco-notification"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// AdminMailTestHandler serves POST /api/v1/admin/mail/test. Body:
//
//	{ "to": "someone@example.com", "name": "Ada" }
//
// It enqueues a MailTask using the "test" template — actual delivery is
// handled by the mail-outbound queue consumer.
type AdminMailTestHandler struct{}

type testRequest struct {
	To   string `json:"to"`
	Name string `json:"name"`
}

// @Summary     Send a test mail
// @Tags        admin-mail
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       body body interface{} true "Request body"
// @Success     200 {object} interface{}
// @Failure     400,401,403,500 {object} map[string]interface{}
// @Router      /admin/mail/test [post]
func (h *AdminMailTestHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()
	ctx := reqCtx.GetDI()

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
		req.Name = "friend"
	}

	bag, ok := ctx.(*di.ContextBag)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "DI context has unexpected type")
		return
	}
	raw, ok := bag.Get(iam_notification.ContextBagKeyService)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "mail service not available")
		return
	}
	svc, ok := raw.(coconotification.Service)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "mail service has unexpected type")
		return
	}

	task := coconotification.Task{
		Template: "test",
		To:       []coconotification.Address{{Email: req.To}},
		Data: map[string]interface{}{
			"Name": req.Name,
			"Time": time.Now().UTC().Format(time.RFC3339),
		},
	}

	id, err := svc.Enqueue(task)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "enqueue failed: "+err.Error())
		return
	}

	response.SuccessResponse(w, http.StatusAccepted, map[string]string{
		"status": "queued",
		"to":     req.To,
		"id":     id,
	})
}
