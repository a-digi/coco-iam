package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/a-digi/coco-iam/src/security/loginbans"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// GetSettingsHandler serves GET /api/v1/admin/security/login-bans/settings.
type GetSettingsHandler struct{}

// @Summary     Get failed-login ban-rule settings
// @Description Returns the current admin-console and application-login failed-login ban-rule settings. Both default to disabled until explicitly turned on.
// @Tags        security-login-bans
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} handler.SettingsSuccess
// @Failure     401,403,500 {object} response.ErrorBody
// @Router      /admin/security/login-bans/settings [get]
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

// PutSettingsHandler serves PUT /api/v1/admin/security/login-bans/settings.
type PutSettingsHandler struct{}

// @Summary     Update failed-login ban-rule settings
// @Description Updates the admin-console and application-login failed-login ban-rule settings. A domain with enabled=false stores its numbers as submitted but skips validation on them, since no live rule depends on them yet.
// @Tags        security-login-bans
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       body body handler.SettingsRequest true "Ban-rule settings"
// @Success     200 {object} handler.SettingsSuccess
// @Failure     400,401,403,500 {object} response.ErrorBody
// @Router      /admin/security/login-bans/settings [put]
func (h *PutSettingsHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	var req SettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	defer r.Body.Close()

	if err := validateDomainRule("admin", req.Admin); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validateDomainRule("application", req.Application); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	persist, ok := resolveSettingsPersist(reqCtx)
	if !ok {
		return
	}
	if err := persist.SaveSettings(loginbans.Settings{
		Admin: loginbans.DomainRule{
			Enabled:       req.Admin.Enabled,
			Threshold:     req.Admin.Threshold,
			WindowSeconds: req.Admin.WindowSeconds,
			BanSeconds:    req.Admin.BanSeconds,
		},
		Application: loginbans.DomainRule{
			Enabled:       req.Application.Enabled,
			Threshold:     req.Application.Threshold,
			WindowSeconds: req.Application.WindowSeconds,
			BanSeconds:    req.Application.BanSeconds,
		},
	}); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Re-read rather than echo the request back, same convention as
	// GeoIP's own PutSettingsHandler.
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

// validateDomainRule only enforces sane numbers on a rule that's
// actually enabled — a disabled rule's stored numbers don't back any
// live check yet, so there's nothing unsafe about leaving them
// unvalidated (an admin toggling a rule on and off while experimenting
// with values shouldn't get blocked on the "off" half).
func validateDomainRule(name string, d DomainRuleResponse) error {
	if !d.Enabled {
		return nil
	}
	if d.Threshold < 1 {
		return fmt.Errorf("%s.threshold must be >= 1", name)
	}
	if d.WindowSeconds <= 0 {
		return fmt.Errorf("%s.window_seconds must be > 0", name)
	}
	if d.BanSeconds <= 0 {
		return fmt.Errorf("%s.ban_seconds must be > 0", name)
	}
	return nil
}
