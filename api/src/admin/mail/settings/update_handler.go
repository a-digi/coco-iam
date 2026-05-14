package settings

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/a-digi/coco-iam/config/di"
	iam_mail "github.com/a-digi/coco-iam/src/mail"
	"github.com/a-digi/coco-iam/src/mail/accounts"
	mailsettings "github.com/a-digi/coco-iam/src/mail/settings"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// AdminMailSettingsUpdateHandler serves PATCH /api/v1/admin/mail/settings.
// Accepts only `events` — SMTP account edits go through the dedicated
// /admin/mail/accounts resource. Sending an `smtp` block yields a 400.
//
// Event bindings require template + account as a pair: both set or both
// empty. A bound account is checked against the accounts store so dangling
// references can't be saved.
type AdminMailSettingsUpdateHandler struct{}

type updateRequest struct {
	SMTP       json.RawMessage             `json:"smtp,omitempty"`
	Events     []mailsettings.EventBinding `json:"events,omitempty"`
	Activation *activationPatch            `json:"activation,omitempty"`
}

type activationPatch struct {
	TTLHours              *int `json:"ttl_hours,omitempty"`
	ResendCooldownSeconds *int `json:"resend_cooldown_seconds,omitempty"`
}

func (h *AdminMailSettingsUpdateHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	store, _ := resolveStoreResolver(reqCtx)
	if store == nil {
		return
	}
	accountsStore := resolveAccountsStore(reqCtx)
	if accountsStore == nil {
		return
	}

	var req updateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	defer r.Body.Close()

	if len(req.SMTP) > 0 && string(req.SMTP) != "null" {
		response.ErrorResponse(w, http.StatusBadRequest,
			"SMTP settings have moved — use /api/v1/admin/mail/accounts to manage accounts")
		return
	}

	if req.Events != nil {
		known := map[string]bool{}
		for _, evt := range mailsettings.EventCatalog {
			known[evt.Key] = true
		}
		updates := map[string]string{}
		for _, binding := range req.Events {
			if !known[binding.Event] {
				response.ErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("unknown event key %q", binding.Event))
				return
			}
			tpl := strings.TrimSpace(binding.Template)
			acc := strings.TrimSpace(binding.Account)

			// Paired requirement: either both set (configured) or both
			// empty (cleared). Partial bindings would cause a send-time
			// error, so reject them up front.
			if (tpl == "") != (acc == "") {
				response.ErrorResponse(w, http.StatusBadRequest,
					fmt.Sprintf("event %q: template and account must both be set or both empty", binding.Event))
				return
			}

			// Verify the referenced account actually exists so bindings
			// can't point at a deleted row.
			if acc != "" {
				exists, err := accountsStore.Exists(acc)
				if err != nil {
					response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
					return
				}
				if !exists {
					response.ErrorResponse(w, http.StatusBadRequest,
						fmt.Sprintf("event %q: account %q does not exist", binding.Event, acc))
					return
				}
			}

			updates[mailsettings.EventTemplateKey(binding.Event)] = tpl
			updates[mailsettings.EventAccountKey(binding.Event)] = acc
		}
		if err := store.SetMany(updates); err != nil {
			response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	if req.Activation != nil {
		updates := map[string]string{}
		if req.Activation.TTLHours != nil {
			if *req.Activation.TTLHours < 1 {
				response.ErrorResponse(w, http.StatusBadRequest,
					"activation.ttl_hours must be >= 1")
				return
			}
			updates[mailsettings.KeyActivationTTLHours] = strconv.Itoa(*req.Activation.TTLHours)
		}
		if req.Activation.ResendCooldownSeconds != nil {
			if *req.Activation.ResendCooldownSeconds < 0 {
				response.ErrorResponse(w, http.StatusBadRequest,
					"activation.resend_cooldown_seconds must be >= 0")
				return
			}
			updates[mailsettings.KeyActivationResendCooldown] = strconv.Itoa(*req.Activation.ResendCooldownSeconds)
		}
		if len(updates) > 0 {
			if err := store.SetMany(updates); err != nil {
				response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
				return
			}
		}
	}

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

// resolveAccountsStore is a small helper so the update handler can
// validate referenced account names without coupling to the other
// accounts admin package.
func resolveAccountsStore(reqCtx request.RequestContext) *accounts.Store {
	w := reqCtx.GetWriter()
	ctx := reqCtx.GetDI()
	bag, ok := ctx.(*di.ContextBag)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "DI context has unexpected type")
		return nil
	}
	raw, ok := bag.Get(iam_mail.ContextBagKeyAccountsStore)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "mail accounts store not available")
		return nil
	}
	s, ok := raw.(*accounts.Store)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "mail accounts store has unexpected type")
		return nil
	}
	return s
}

