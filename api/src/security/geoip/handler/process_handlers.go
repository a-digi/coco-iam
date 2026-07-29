package handler

import (
	"net/http"
	"time"

	"github.com/a-digi/coco-iam/src/security/geoip"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// StartHandler serves POST /api/v1/admin/security/geoip/start.
type StartHandler struct{}

// @Summary     Start the geoip-updater process
// @Description Launches the geoip-updater executable, detached so it survives
// @Description this admin server's own restart. Refuses (409) if an instance
// @Description is already running — tracked via a PID file, not in-memory
// @Description state, so this is accurate even across an admin-server restart.
// @Description See plan/geoip-enrichment/plan.md's "Process control" section.
// @Tags        security-geoip
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} handler.StatusSuccess
// @Failure     401,403,409,500 {object} response.ErrorBody
// @Router      /admin/security/geoip/start [post]
func (h *StartHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	manager, ok := resolveManager(reqCtx)
	if !ok {
		return
	}
	pid, err := manager.Start()
	if err != nil {
		response.ErrorResponse(w, http.StatusConflict, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, StatusResponse{Running: true, PID: pid})
}

// StopHandler serves POST /api/v1/admin/security/geoip/stop.
type StopHandler struct{}

// @Summary     Stop the geoip-updater process
// @Description Signals the running geoip-updater process to shut down
// @Description gracefully. A no-op (not an error) if nothing is currently
// @Description running.
// @Tags        security-geoip
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} handler.StatusSuccess
// @Failure     401,403,500 {object} response.ErrorBody
// @Router      /admin/security/geoip/stop [post]
func (h *StopHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	manager, ok := resolveManager(reqCtx)
	if !ok {
		return
	}
	if err := manager.Stop(); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, StatusResponse{Running: false})
}

// StatusHandler serves GET /api/v1/admin/security/geoip/status.
type StatusHandler struct{}

// @Summary     GeoIP updater process status
// @Description Reports whether the geoip-updater process is currently
// @Description running, its PID, whether geoip is enabled, and when it last
// @Description successfully pulled fresh data (if ever).
// @Tags        security-geoip
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} handler.StatusSuccess
// @Failure     401,403,500 {object} response.ErrorBody
// @Router      /admin/security/geoip/status [get]
func (h *StatusHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	manager, ok := resolveManager(reqCtx)
	if !ok {
		return
	}
	status, err := manager.Status()
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	query, ok := resolveSettingsQuery(reqCtx)
	if !ok {
		return
	}
	settings, err := query.LoadSettings()
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	cfg := geoip.DefaultConfig().WithSettings(settings)

	resp := StatusResponse{Running: status.Running, PID: status.PID, Enabled: cfg.Enabled}
	if !status.LastPulledAt.IsZero() {
		resp.LastPulledAt = status.LastPulledAt.UTC().Format(time.RFC3339)
	}
	response.SuccessResponse(w, http.StatusOK, resp)
}
