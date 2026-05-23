package accounts

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/a-digi/coco-iam/config/di"
	"github.com/a-digi/coco-lift/resource/uri"
	iam_mail "github.com/a-digi/coco-iam/src/mail"
	mailaccounts "github.com/a-digi/coco-iam/src/mail/accounts"
	mailsettings "github.com/a-digi/coco-iam/src/mail/settings"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// AdminMailAccountsDeleteHandler serves
// DELETE /api/v1/admin/mail/accounts/{id}. Refuses in two cases:
//   - Account is currently active (409, "activate another first").
//   - Account is referenced by one or more event bindings (409, names the events).
type AdminMailAccountsDeleteHandler struct{}

// @Summary     Delete a mail account
// @Tags        admin-mail
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Account ID"
// @Success     200 {object} interface{}
// @Failure     400,401,403,500 {object} map[string]interface{}
// @Router      /admin/mail/accounts/{id} [delete]
func (h *AdminMailAccountsDeleteHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()
	key, value := uri.ExtractKeyAndValueFromURI(r.URL.Path)
	if key != "id" || value == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "account id is required")
		return
	}
	store := resolveStore(reqCtx)
	if store == nil {
		return
	}

	acc, err := store.Get(value)
	if err != nil {
		if errors.Is(err, mailaccounts.ErrNotFound) {
			response.ErrorResponse(w, http.StatusNotFound, "account not found")
			return
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	if acc.IsActive {
		response.ErrorResponse(w, http.StatusConflict,
			"cannot delete the active account — activate another account first")
		return
	}

	// Event-binding guard. If any `event.<key>.account` row points at this
	// account's name, block deletion and tell the admin which events to
	// rebind first.
	settingsStore := resolveSettingsStore(reqCtx)
	if settingsStore == nil {
		return
	}
	keys, err := settingsStore.KeysWithValue("event.", acc.Name)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	if len(keys) > 0 {
		events := make([]string, 0, len(keys))
		for _, k := range keys {
			if strings.HasSuffix(k, ".account") {
				eventKey := strings.TrimSuffix(strings.TrimPrefix(k, "event."), ".account")
				if eventKey != "" {
					events = append(events, eventKey)
				}
			}
		}
		if len(events) > 0 {
			response.ErrorResponse(w, http.StatusConflict,
				fmt.Sprintf("cannot delete account %q — bound to event(s): %s. Rebind those events first.",
					acc.Name, strings.Join(events, ", ")))
			return
		}
	}

	if err := store.Delete(value); err != nil {
		switch {
		case errors.Is(err, mailaccounts.ErrNotFound):
			response.ErrorResponse(w, http.StatusNotFound, "account not found")
		case errors.Is(err, mailaccounts.ErrActiveAccount):
			response.ErrorResponse(w, http.StatusConflict,
				"cannot delete the active account — activate another account first")
		default:
			response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	response.SuccessResponse(w, http.StatusOK, map[string]string{"id": value, "status": "deleted"})
}

func resolveSettingsStore(reqCtx request.RequestContext) *mailsettings.Store {
	w := reqCtx.GetWriter()
	ctx := reqCtx.GetDI()
	bag, ok := ctx.(*di.ContextBag)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "DI context has unexpected type")
		return nil
	}
	raw, ok := bag.Get(iam_mail.ContextBagKeySettingsStore)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "mail settings store not available")
		return nil
	}
	s, ok := raw.(*mailsettings.Store)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "mail settings store has unexpected type")
		return nil
	}
	return s
}
