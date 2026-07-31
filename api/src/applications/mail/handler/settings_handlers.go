package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/a-digi/coco-iam/src/applications/mail/entity"
	appmail_persistent "github.com/a-digi/coco-iam/src/applications/mail/repository/persistent"
	appmail_query "github.com/a-digi/coco-iam/src/applications/mail/repository/query"
	mailsettings "github.com/a-digi/coco-iam/src/notification"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// buildAppMailSettingsResponse loads exactly what this application has
// stored — its own active account (nil if none), its own event
// bindings (empty string = not customized here), and its own
// activation overrides (nil = not customized here). Not a resolved
// cascade view; that lives in api/src/mail/scopedsettings, used at
// send time.
func buildAppMailSettingsResponse(db *sql.DB, appID string) (entity.AppMailSettingsResponse, error) {
	accountsRepo := appmail_query.NewAppMailAccountsQueryRepo(db, appID)
	settingsRepo := appmail_query.NewAppMailSettingsQueryRepo(db, appID)

	out := entity.AppMailSettingsResponse{
		Events: make([]entity.AppEventBinding, 0, len(mailsettings.EventCatalog)),
	}

	active, err := accountsRepo.GetActive()
	if err != nil && !errors.Is(err, appmail_query.ErrNoActive) {
		return entity.AppMailSettingsResponse{}, err
	}
	if active != nil {
		resp := toAccountResponse(*active)
		out.ActiveAccount = &resp
	}

	for _, evt := range mailsettings.EventCatalog {
		tpl, _, _ := settingsRepo.Get(appmail_query.EventTemplateKey(evt.Key))
		acc, _, _ := settingsRepo.Get(appmail_query.EventAccountKey(evt.Key))
		out.Events = append(out.Events, entity.AppEventBinding{Event: evt.Key, Template: tpl, Account: acc})
	}

	if v, ok, _ := settingsRepo.Get(appmail_query.KeyActivationTTLHours); ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			out.Activation.TTLHours = &n
		}
	}
	if v, ok, _ := settingsRepo.Get(appmail_query.KeyActivationResendCooldown); ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			out.Activation.ResendCooldownSeconds = &n
		}
	}
	return out, nil
}

// AppMailSettingsGetHandler serves GET /applications/{id}/mail/settings.
type AppMailSettingsGetHandler struct{}

// @Summary     Get an application's mail settings
// @Description Returns exactly what this application has customized — active account, event
// @Description bindings, and activation overrides. Empty/nil fields fall back to the
// @Description organization's, then the global, mail engine settings at send time.
// @Tags        app-mail
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Application ID"
// @Success     200 {object} entity.AppMailSettingsSuccess
// @Failure     400,401,403,404,500 {object} response.ErrorBody
// @Router      /applications/{id}/mail/settings [get]
func (h *AppMailSettingsGetHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	db, appID, ok := resolveAppDB(reqCtx)
	if !ok {
		return
	}
	snap, err := buildAppMailSettingsResponse(db, appID)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, snap)
}

// AppMailSettingsUpdateHandler serves PATCH /applications/{id}/mail/settings.
type AppMailSettingsUpdateHandler struct{}

// @Summary     Update an application's mail settings
// @Description Event bindings require template + account as a pair — both set (bound to
// @Description this application's own account/template) or both empty (clears the
// @Description application override, falls back to the organization's, then the global,
// @Description binding). A bound account must exist among this application's own accounts.
// @Tags        app-mail
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Application ID"
// @Param       body body entity.AppMailSettingsUpdateRequest true "Patch"
// @Success     200 {object} entity.AppMailSettingsSuccess
// @Failure     400,401,403,404,500 {object} response.ErrorBody
// @Router      /applications/{id}/mail/settings [patch]
func (h *AppMailSettingsUpdateHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	db, appID, ok := resolveAppDB(reqCtx)
	if !ok {
		return
	}

	var req entity.AppMailSettingsUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	defer r.Body.Close()

	settingsPersist := appmail_persistent.NewAppMailSettingsPersistentRepo(db, appID)

	if req.Events != nil {
		known := map[string]bool{}
		for _, evt := range mailsettings.EventCatalog {
			known[evt.Key] = true
		}
		accountsQuery := appmail_query.NewAppMailAccountsQueryRepo(db, appID)
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
						fmt.Sprintf("event %q: account %q does not exist in this application", binding.Event, acc))
					return
				}
			}
			updates[appmail_query.EventTemplateKey(binding.Event)] = tpl
			updates[appmail_query.EventAccountKey(binding.Event)] = acc
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
			updates[appmail_query.KeyActivationTTLHours] = strconv.Itoa(*req.Activation.TTLHours)
		}
		if req.Activation.ResendCooldownSeconds != nil {
			if *req.Activation.ResendCooldownSeconds < 0 {
				response.ErrorResponse(w, http.StatusBadRequest, "activation.resend_cooldown_seconds must be >= 0")
				return
			}
			updates[appmail_query.KeyActivationResendCooldown] = strconv.Itoa(*req.Activation.ResendCooldownSeconds)
		}
		if len(updates) > 0 {
			if err := settingsPersist.SetMany(updates); err != nil {
				response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
				return
			}
		}
	}

	snap, err := buildAppMailSettingsResponse(db, appID)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.SuccessResponse(w, http.StatusOK, snap)
}
