package settings

import (
	"net/http"

	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// AdminMailSettingsGetHandler serves GET /api/v1/admin/mail/settings.
// Returns the currently active SMTP account + every event binding in the
// catalog. The account password is redacted.
type AdminMailSettingsGetHandler struct{}

// @Summary     Get mail settings
// @Tags        admin-mail
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} interface{}
// @Failure     400,401,403,500 {object} map[string]interface{}
// @Router      /admin/mail/settings [get]
func (h *AdminMailSettingsGetHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	_, resolver := resolveStoreResolver(reqCtx)
	if resolver == nil {
		return
	}
	snap := resolver.Snapshot()
	if snap.ActiveAccount != nil {
		redacted := snap.ActiveAccount.Redacted()
		snap.ActiveAccount = &redacted
	}
	response.SuccessResponse(w, http.StatusOK, snap)
}
