package admin

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/a-digi/coco-iam/src/general"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// AdminGeneralSettingsUpdateHandler serves PATCH /api/v1/admin/settings/general.
// Any field may be omitted; a present string replaces the stored value.
type AdminGeneralSettingsUpdateHandler struct{}

// @Summary     Update general settings
// @Tags        admin-settings
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} interface{}
// @Failure     400,401,403,500 {object} map[string]interface{}
// @Router      /admin/settings/general [patch]
func (h *AdminGeneralSettingsUpdateHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	store := resolveGlobalStore(reqCtx, w)
	if store == nil {
		return
	}

	var req updateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	defer r.Body.Close()

	updates := map[string]string{}

	if req.BaseURL != nil {
		v := strings.TrimSpace(*req.BaseURL)
		if v != "" {
			if !strings.HasPrefix(v, "http://") && !strings.HasPrefix(v, "https://") {
				response.ErrorResponse(w, http.StatusBadRequest,
					"base_url must start with http:// or https://")
				return
			}
			if !isAllowedBaseURLHost(v) {
				response.ErrorResponse(w, http.StatusBadRequest,
					"base_url host is not in the configured allowlist")
				return
			}
		}
		updates[general.KeyBaseURL] = strings.TrimRight(v, "/")
	}
	if req.PageTitle != nil {
		updates[general.KeyPageTitle] = strings.TrimSpace(*req.PageTitle)
	}
	if req.Description != nil {
		updates[general.KeyDescription] = strings.TrimSpace(*req.Description)
	}
	if req.Robots != nil {
		updates[general.KeyRobots] = strings.TrimSpace(*req.Robots)
	}

	if len(updates) > 0 {
		if err := store.SetMany(updates); err != nil {
			response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	snap, err := store.Snapshot()
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, snap)
}

// isAllowedBaseURLHost checks rawURL's host against
// ALLOWED_FRONTEND_BASE_URL_HOSTS, a comma-separated allowlist an
// operator opts into at deploy time. Unset (the default) means no
// restriction — base_url is legitimately allowed to point at a
// different domain than COCO_IAM_PUBLIC_BASE_URL (frontend and API
// can live on different subdomains), so this can't just reuse that
// value; it needs its own explicit opt-in list. Once set, this is
// what stops an admin (or a compromised admin session) from pointing
// activation/recovery email links at an attacker-controlled domain.
// See plan/todo/security/header-and-cache-poisoning.md.
func isAllowedBaseURLHost(rawURL string) bool {
	allowlist := strings.TrimSpace(os.Getenv("ALLOWED_FRONTEND_BASE_URL_HOSTS"))
	if allowlist == "" {
		return true
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return false
	}
	for _, allowed := range strings.Split(allowlist, ",") {
		if strings.EqualFold(strings.TrimSpace(allowed), u.Host) {
			return true
		}
	}
	return false
}
