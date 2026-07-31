// Package accounts hosts the admin HTTP handlers for mail_smtp_accounts
// (list, create, update, delete, activate, test).
package accounts

import (
	"net/http"

	"github.com/a-digi/coco-iam/config/di"
	iam_mail "github.com/a-digi/coco-iam/src/notification"
	mailaccounts "github.com/a-digi/coco-notification/mailer"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// resolveStore fetches the accounts Store from DI. Writes an error response
// and returns nil on any lookup failure so the caller can early-return.
func resolveStore(reqCtx request.RequestContext) *mailaccounts.Store {
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
	store, ok := raw.(*mailaccounts.Store)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "mail accounts store has unexpected type")
		return nil
	}
	return store
}
