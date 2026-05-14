// Package settings hosts the admin HTTP handlers for the mail settings
// surface (SMTP engine config + event→template bindings).
package settings

import (
	"net/http"

	"github.com/a-digi/coco-iam/config/di"
	iam_mail "github.com/a-digi/coco-iam/src/mail"
	mailsettings "github.com/a-digi/coco-iam/src/mail/settings"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// resolveStoreResolver returns both services (store + resolver) from the
// DI bag. Writes an error response and returns nil,nil if anything's
// missing so callers can early-return.
func resolveStoreResolver(reqCtx request.RequestContext) (*mailsettings.Store, *mailsettings.Resolver) {
	w := reqCtx.GetWriter()
	ctx := reqCtx.GetDI()
	bag, ok := ctx.(*di.ContextBag)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "DI context has unexpected type")
		return nil, nil
	}
	rawStore, ok := bag.Get(iam_mail.ContextBagKeySettingsStore)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "mail settings store not available")
		return nil, nil
	}
	store, ok := rawStore.(*mailsettings.Store)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "mail settings store has unexpected type")
		return nil, nil
	}
	rawResolver, ok := bag.Get(iam_mail.ContextBagKeySettingsResolver)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "mail settings resolver not available")
		return nil, nil
	}
	resolver, ok := rawResolver.(*mailsettings.Resolver)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "mail settings resolver has unexpected type")
		return nil, nil
	}
	return store, resolver
}
