package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/a-digi/coco-iam/src/security/geoip"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// GetSettingsHandler serves GET /api/v1/admin/security/geoip/settings.
type GetSettingsHandler struct{}

// @Summary     Get GeoIP settings
// @Description Returns the current GeoIP enrichment settings. The MaxMind
// @Description license key is never returned — only a fixed mask when one is
// @Description stored. See plan/geoip-enrichment/plan.md.
// @Tags        security-geoip
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} handler.SettingsSuccess
// @Failure     401,403,500 {object} response.ErrorBody
// @Router      /admin/security/geoip/settings [get]
func (h *GetSettingsHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	query, ok := resolveSettingsQuery(reqCtx)
	if !ok {
		return
	}
	resp, err := resolvedSettingsResponse(query)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, resp)
}

// PutSettingsHandler serves PUT /api/v1/admin/security/geoip/settings.
type PutSettingsHandler struct{}

// @Summary     Update GeoIP settings
// @Description Updates GeoIP enrichment settings, including the MaxMind
// @Description account ID and license key. An empty/omitted
// @Description maxmind_license_key leaves the currently-stored key unchanged
// @Description — submit a non-empty value only when actually changing it.
// @Description Changes take effect the next time the geoip-updater process is
// @Description (re)started, not while it's already running.
// @Tags        security-geoip
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       body body handler.SettingsRequest true "GeoIP settings"
// @Success     200 {object} handler.SettingsSuccess
// @Failure     400,401,403,500 {object} response.ErrorBody
// @Router      /admin/security/geoip/settings [put]
func (h *PutSettingsHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	var req SettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	defer r.Body.Close()

	req.MaxMindAccountID = strings.TrimSpace(req.MaxMindAccountID)
	if req.CheckIntervalSeconds <= 0 {
		response.ErrorResponse(w, http.StatusBadRequest, "check_interval_seconds must be > 0")
		return
	}
	if req.PullIntervalHours <= 0 {
		response.ErrorResponse(w, http.StatusBadRequest, "pull_interval_hours must be > 0")
		return
	}

	persist, ok := resolveSettingsPersist(reqCtx)
	if !ok {
		return
	}
	if err := persist.SaveSettings(geoip.Settings{
		Enabled:              req.Enabled,
		MaxMindAccountID:     req.MaxMindAccountID,
		MaxMindLicenseKey:    req.MaxMindLicenseKey,
		CheckIntervalSeconds: req.CheckIntervalSeconds,
		PullIntervalHours:    req.PullIntervalHours,
	}); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Re-read rather than echo the request back — the response must
	// reflect what was actually stored (e.g. whether a key ended up
	// present), not just what was submitted.
	query, ok := resolveSettingsQuery(reqCtx)
	if !ok {
		return
	}
	resp, err := resolvedSettingsResponse(query)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, resp)
}
