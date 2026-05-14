package admin

import (
	"net/http"

	"github.com/a-digi/coco-iam/config/di"
	"github.com/a-digi/coco-iam/src/mail/consumer"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// AdminMailStatusHandler serves GET /api/v1/admin/mail/status. Returns
// `{backlog, workers, min, max, step}` — the orchestrator's latest sample.
type AdminMailStatusHandler struct{}

func (h *AdminMailStatusHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	ctx := reqCtx.GetDI()

	bag, ok := ctx.(*di.ContextBag)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "DI context has unexpected type")
		return
	}
	raw, ok := bag.Get(consumer.ContextBagKeyOrchestrator)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "mail orchestrator not available")
		return
	}
	orch, ok := raw.(*consumer.Orchestrator)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "mail orchestrator has unexpected type")
		return
	}
	response.SuccessResponse(w, http.StatusOK, orch.Current())
}
