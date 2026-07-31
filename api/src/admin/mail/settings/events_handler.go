package settings

import (
	"net/http"

	iam_notification "github.com/a-digi/coco-iam/src/notification"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// AdminMailSettingsEventsHandler serves
// GET /api/v1/admin/mail/settings/events.
// Returns the backend-owned list of events that support a template
// binding. The UI renders one dropdown per entry.
type AdminMailSettingsEventsHandler struct{}

// @Summary     List mail settings event catalog
// @Tags        admin-mail
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} interface{}
// @Failure     400,401,403,500 {object} map[string]interface{}
// @Router      /admin/mail/settings/events [get]
func (h *AdminMailSettingsEventsHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	response.SuccessResponse(w, http.StatusOK, iam_notification.EventCatalog)
}
