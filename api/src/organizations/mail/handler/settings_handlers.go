package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	mailsettings "github.com/a-digi/coco-iam/src/mail/settings"
	"github.com/a-digi/coco-iam/src/organizations/mail/entity"
	orgmail_persistent "github.com/a-digi/coco-iam/src/organizations/mail/repository/persistent"
	orgmail_query "github.com/a-digi/coco-iam/src/organizations/mail/repository/query"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// buildOrgMailSettingsResponse loads exactly what this org has stored —
// its own active account (nil if none), its own event bindings (empty
// string = not customized here), and its own activation overrides
// (nil = not customized here). Not a resolved cascade view; that lives
// in api/src/mail/scopedsettings, used at send time.
func buildOrgMailSettingsResponse(db *sql.DB) (entity.OrgMailSettingsResponse, error) {
	accountsRepo := orgmail_query.NewOrgMailAccountsQueryRepo(db)
	settingsRepo := orgmail_query.NewOrgMailSettingsQueryRepo(db)

	out := entity.OrgMailSettingsResponse{
		Events: make([]entity.OrgEventBinding, 0, len(mailsettings.EventCatalog)),
	}

	active, err := accountsRepo.GetActive()
	if err != nil && !errors.Is(err, orgmail_query.ErrNoActive) {
		return entity.OrgMailSettingsResponse{}, err
	}
	if active != nil {
		resp := toAccountResponse(*active)
		out.ActiveAccount = &resp
	}

	for _, evt := range mailsettings.EventCatalog {
		tpl, _, _ := settingsRepo.Get(orgmail_query.EventTemplateKey(evt.Key))
		acc, _, _ := settingsRepo.Get(orgmail_query.EventAccountKey(evt.Key))
		out.Events = append(out.Events, entity.OrgEventBinding{Event: evt.Key, Template: tpl, Account: acc})
	}

	if v, ok, _ := settingsRepo.Get(orgmail_query.KeyActivationTTLHours); ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			out.Activation.TTLHours = &n
		}
	}
	if v, ok, _ := settingsRepo.Get(orgmail_query.KeyActivationResendCooldown); ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			out.Activation.ResendCooldownSeconds = &n
		}
	}
	return out, nil
}

// OrgMailSettingsGetHandler serves GET /organizations/{id}/mail/settings.
type OrgMailSettingsGetHandler struct{}

// @Summary     Get an organization's mail settings
// @Description Returns exactly what this org has customized — active account, event
// @Description bindings, and activation overrides. Empty/nil fields fall back to the
// @Description global mail engine settings at send time.
// @Tags        org-mail
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Organization ID"
// @Success     200 {object} entity.OrgMailSettingsSuccess
// @Failure     400,401,403,404,500 {object} response.ErrorBody
// @Router      /organizations/{id}/mail/settings [get]
func (h *OrgMailSettingsGetHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	db, ok := resolveOrgDB(reqCtx)
	if !ok {
		return
	}
	snap, err := buildOrgMailSettingsResponse(db)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, snap)
}

// OrgMailSettingsUpdateHandler serves PATCH /organizations/{id}/mail/settings.
type OrgMailSettingsUpdateHandler struct{}

// @Summary     Update an organization's mail settings
// @Description Event bindings require template + account as a pair — both set (bound to
// @Description this org's own account/template) or both empty (clears the org override,
// @Description falls back to the global binding). A bound account must exist among this
// @Description org's own accounts.
// @Tags        org-mail
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Organization ID"
// @Param       body body entity.OrgMailSettingsUpdateRequest true "Patch"
// @Success     200 {object} entity.OrgMailSettingsSuccess
// @Failure     400,401,403,404,500 {object} response.ErrorBody
// @Router      /organizations/{id}/mail/settings [patch]
func (h *OrgMailSettingsUpdateHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	db, ok := resolveOrgDB(reqCtx)
	if !ok {
		return
	}

	var req entity.OrgMailSettingsUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	defer r.Body.Close()

	settingsPersist := orgmail_persistent.NewOrgMailSettingsPersistentRepo(db)

	if req.Events != nil {
		known := map[string]bool{}
		for _, evt := range mailsettings.EventCatalog {
			known[evt.Key] = true
		}
		accountsQuery := orgmail_query.NewOrgMailAccountsQueryRepo(db)
		updates := map[string]string{}
		for _, binding := range req.Events {
			if !known[binding.Event] {
				response.ErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("unknown event key %q", binding.Event))
				return
			}
			tpl := strings.TrimSpace(binding.Template)
			acc := strings.TrimSpace(binding.Account)
			if (tpl == "") != (acc == "") {
				response.ErrorResponse(w, http.StatusBadRequest,
					fmt.Sprintf("event %q: template and account must both be set or both empty", binding.Event))
				return
			}
			if acc != "" {
				exists, err := accountsQuery.Exists(acc)
				if err != nil {
					response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
					return
				}
				if !exists {
					response.ErrorResponse(w, http.StatusBadRequest,
						fmt.Sprintf("event %q: account %q does not exist in this organization", binding.Event, acc))
					return
				}
			}
			updates[orgmail_query.EventTemplateKey(binding.Event)] = tpl
			updates[orgmail_query.EventAccountKey(binding.Event)] = acc
		}
		if err := settingsPersist.SetMany(updates); err != nil {
			response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	if req.Activation != nil {
		updates := map[string]string{}
		if req.Activation.TTLHours != nil {
			if *req.Activation.TTLHours < 1 {
				response.ErrorResponse(w, http.StatusBadRequest, "activation.ttl_hours must be >= 1")
				return
			}
			updates[orgmail_query.KeyActivationTTLHours] = strconv.Itoa(*req.Activation.TTLHours)
		}
		if req.Activation.ResendCooldownSeconds != nil {
			if *req.Activation.ResendCooldownSeconds < 0 {
				response.ErrorResponse(w, http.StatusBadRequest, "activation.resend_cooldown_seconds must be >= 0")
				return
			}
			updates[orgmail_query.KeyActivationResendCooldown] = strconv.Itoa(*req.Activation.ResendCooldownSeconds)
		}
		if len(updates) > 0 {
			if err := settingsPersist.SetMany(updates); err != nil {
				response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
				return
			}
		}
	}

	snap, err := buildOrgMailSettingsResponse(db)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, snap)
}
