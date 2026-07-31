// Package settings hosts the admin HTTP handlers for the mail settings
// surface (SMTP engine config + event→template bindings).
package settings

import (
	"net/http"

	"github.com/a-digi/coco-iam/config/di"
	iam_mail "github.com/a-digi/coco-iam/src/notification"
	"github.com/a-digi/coco-notification/mailer"
	mailsettings "github.com/a-digi/coco-notification/settings"
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

// mailSettingsSnapshot is the admin GET/PATCH /admin/mail/settings
// response shape — same field names as the old, now-retired
// notsettings.Resolver.Snapshot() (removed when the mail engine
// extracted into the generic coco-notification library, since
// building a snapshot requires iterating an event catalog, and the
// generic library deliberately doesn't ship one — see
// iam_notification.EventCatalog).
type mailSettingsSnapshot struct {
	ActiveAccount *mailer.Account                 `json:"active_account"`
	Events        []mailsettings.EventBinding     `json:"events"`
	Activation    mailsettings.ActivationSettings `json:"activation"`
}

// buildMailSettingsSnapshot loads exactly what the global tier has
// configured — mirrors organizations/mail/handler's own
// buildOrgMailSettingsResponse (which already builds its own view
// the same way, never having depended on Resolver.Snapshot() itself).
func buildMailSettingsSnapshot(resolver *mailsettings.Resolver) mailSettingsSnapshot {
	out := mailSettingsSnapshot{
		ActiveAccount: resolver.ActiveAccount(),
		Events:        make([]mailsettings.EventBinding, 0, len(iam_mail.EventCatalog)),
		Activation:    resolver.ActivationSettings(),
	}
	for _, evt := range iam_mail.EventCatalog {
		out.Events = append(out.Events, mailsettings.EventBinding{
			Event:    evt.Key,
			Template: resolver.TemplateForEvent(evt.Key),
			Account:  resolver.AccountForEvent(evt.Key),
		})
	}
	return out
}
