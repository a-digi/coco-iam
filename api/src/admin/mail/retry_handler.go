package admin

import (
	"net/http"

	"github.com/a-digi/coco-iam/config/di"
	"github.com/a-digi/coco-lift/resource/uri"
	iam_mail "github.com/a-digi/coco-iam/src/mail"
	"github.com/a-digi/coco-iam/src/mail/store"
	"github.com/a-digi/coco-queue"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// AdminMailRetryHandler serves POST /api/v1/admin/mail/outbound/{id}/retry.
// Flips a terminal (failed / dead_lettered) row back to queued and asks the
// queue to re-dispatch via Retry. Because mail_outbound.id == queue_tasks.id
// the two sides stay aligned.
type AdminMailRetryHandler struct{}

func (h *AdminMailRetryHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()
	ctx := reqCtx.GetDI()

	key, value := uri.ExtractKeyAndValueFromURI(r.URL.Path)
	if key != "id" || value == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "mail id is required")
		return
	}

	bag, ok := ctx.(*di.ContextBag)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "DI context has unexpected type")
		return
	}
	rawStore, ok := bag.Get(iam_mail.ContextBagKeyMailStore)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "mail store not available")
		return
	}
	st, ok := rawStore.(*store.Store)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "mail store has unexpected type")
		return
	}
	rawQueue, ok := bag.Get(queue.ContextBagKey)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "queue manager not available")
		return
	}
	mgr, ok := rawQueue.(queue.Manager)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "queue manager has unexpected type")
		return
	}

	if err := st.Requeue(value); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := mgr.Retry(value); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, map[string]string{"id": value, "status": "queued"})
}
