package handler

import (
	"errors"
	"net/http"

	"github.com/a-digi/coco-iam/src/security/geoip"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// SyncHandler serves POST /api/v1/admin/security/geoip/sync.
type SyncHandler struct{}

// @Summary     Force an immediate GeoIP data sync
// @Description Signals the running geoip-updater process to pull fresh data
// @Description immediately, bypassing the normal pull_interval_hours staleness
// @Description check. Fails with 409 if the updater isn't currently running —
// @Description nothing to signal. See plan/geoip-enrichment/plan.md's
// @Description "Extension: database stats + manual sync" section.
// @Tags        security-geoip
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} handler.StatusSuccess
// @Failure     401,403,409,500 {object} response.ErrorBody
// @Router      /admin/security/geoip/sync [post]
func (h *SyncHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	manager, ok := resolveManager(reqCtx)
	if !ok {
		return
	}
	if err := manager.SyncNow(); err != nil {
		if errors.Is(err, geoip.ErrNotRunning) {
			response.ErrorResponse(w, http.StatusConflict, err.Error())
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	resp, ok := resolveStatusResponse(reqCtx, manager)
	if !ok {
		return
	}
	response.SuccessResponse(w, http.StatusOK, resp)
}
