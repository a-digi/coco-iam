package settings

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

// AdminMailSettingsTestHandler serves
// POST /api/v1/admin/mail/settings/test. Identical behaviour to the
// generic /admin/mail/test endpoint, but mounted alongside settings so
// the UI's "Test" button has a single obvious target.
type AdminMailSettingsTestHandler struct{}

type testRequest struct {
	To   string `json:"to"`
	Name string `json:"name"`
}

// @Summary     Send a test mail via settings
// @Tags        admin-mail
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       body body interface{} true "Request body"
// @Success     200 {object} interface{}
// @Failure     400,401,403,500 {object} map[string]interface{}
// @Router      /admin/mail/settings/test [post]
func (h *AdminMailSettingsTestHandler) ServeHTTP(reqCtx request.RequestContext) {
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
		req.Name = "admin"
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
