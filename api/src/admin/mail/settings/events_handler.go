package settings

import (
	"net/http"

	mailsettings "github.com/a-digi/coco-iam/src/mail/settings"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// AdminMailSettingsEventsHandler serves
// GET /api/v1/admin/mail/settings/events.
// Returns the backend-owned list of events that support a template
// binding. The UI renders one dropdown per entry.
type AdminMailSettingsEventsHandler struct{}

func (h *AdminMailSettingsEventsHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	response.SuccessResponse(w, http.StatusOK, mailsettings.EventCatalog)
}
