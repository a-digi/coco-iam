package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/a-digi/coco-sec/attackbans"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// GetSettingsHandler serves GET /api/v1/admin/security/attack-bans/settings.
type GetSettingsHandler struct{}

// @Summary     Get attack ban-rule settings
// @Description Returns the current global ban-rule settings for high-volume scan/probe traffic against nonexistent routes. Disabled until explicitly turned on.
// @Tags        security-attack-bans
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} handler.SettingsSuccess
// @Failure     401,403,500 {object} response.ErrorBody
// @Router      /admin/security/attack-bans/settings [get]
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

// PutSettingsHandler serves PUT /api/v1/admin/security/attack-bans/settings.
type PutSettingsHandler struct{}

// @Summary     Update attack ban-rule settings
// @Description Updates the global ban-rule settings for high-volume scan/probe traffic against nonexistent routes. When disabled, submitted numbers are stored but skip validation, since no live rule depends on them yet.
// @Tags        security-attack-bans
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       body body handler.SettingsRequest true "Ban-rule settings"
// @Success     200 {object} handler.SettingsSuccess
// @Failure     400,401,403,500 {object} response.ErrorBody
// @Router      /admin/security/attack-bans/settings [put]
func (h *PutSettingsHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	var req SettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	defer r.Body.Close()

	if err := validateSettings(req); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	persist, ok := resolveSettingsPersist(reqCtx)
	if !ok {
		return
	}
	if err := persist.SaveSettings(attackbans.Settings{
		Enabled:       req.Enabled,
		Threshold:     req.Threshold,
		WindowSeconds: req.WindowSeconds,
		BanSeconds:    req.BanSeconds,
	}); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Re-read rather than echo the request back, same convention as
	// loginbans' own PutSettingsHandler.
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

// validateSettings only enforces sane numbers when the rule is
// actually enabled — an admin toggling it on and off while
// experimenting with values shouldn't get blocked on the "off" half.
func validateSettings(s SettingsRequest) error {
	if !s.Enabled {
		return nil
	}
	if s.Threshold < 1 {
		return fmt.Errorf("threshold must be >= 1")
	}
	if s.WindowSeconds <= 0 {
		return fmt.Errorf("window_seconds must be > 0")
	}
	if s.BanSeconds <= 0 {
		return fmt.Errorf("ban_seconds must be > 0")
	}
	return nil
}
