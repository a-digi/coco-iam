// Package scopedsettings extends the global mail engine's
// settings.Resolver with organization- and application-level
// overrides. Resolution order: application → organization → global,
// falling through on empty/absent exactly like general.Store.BaseURL()
// and userrules.Store.GetForUser already do elsewhere in this
// codebase.
//
// Step 1 wired only the organization tier and this resolver's own
// logic; step 2 wired it into the real send call sites (activation,
// recovery, notify). Step 4 wires the application tier into this
// resolver's own logic (this file); wiring it into real app-scoped
// send call sites (application login/recovery) is deferred to step 5,
// mirroring exactly how the org tier's consumer-side wiring landed one
// step after its resolver logic. See plan/org-app-email-settings/plan.md.
package scopedsettings

import (
	"database/sql"
	"errors"
	"strconv"

	appmail_query "github.com/a-digi/coco-iam/src/applications/mail/repository/query"
	iam_mail "github.com/a-digi/coco-iam/src/mail"
	mailsettings "github.com/a-digi/coco-iam/src/mail/settings"
	mailsmtp "github.com/a-digi/coco-iam/src/mail/smtp"
	mailtemplate "github.com/a-digi/coco-iam/src/mail/template"
	orgmail_query "github.com/a-digi/coco-iam/src/organizations/mail/repository/query"
	"github.com/a-digi/coco-iam/src/organizations/users/dbregistry"
	"github.com/a-digi/coco-iam/src/orgrouter"
	"github.com/a-digi/coco-logger/logger"
)

// ContextBagKey is where a single *ScopedResolver is stashed in the DI
// ContextBag — mirrors the global mail package's own ContextBagKey*
// constants.
const ContextBagKey = "mail.scoped_settings_resolver"

// ScopedResolver wraps the existing global Resolver — global behavior
// is completely unchanged when orgID (and, later, appID) is empty, so
// every call site keeps working with zero changes until explicitly
// upgraded to pass an org (and later app) id.
type ScopedResolver struct {
	global *mailsettings.Resolver
	orgReg *dbregistry.OrgUserDBRegistry
	log    logger.Logger
}

// NewScopedResolver wires the org tier on top of the existing global
// resolver. orgReg may be nil — every method degrades to the global
// resolver only, same as passing an empty orgID.
func NewScopedResolver(global *mailsettings.Resolver, orgReg *dbregistry.OrgUserDBRegistry, log logger.Logger) *ScopedResolver {
	return &ScopedResolver{global: global, orgReg: orgReg, log: log}
}

// Config resolves the SMTP config to use: the application's own active
// account wins if set, otherwise the org's own active account, otherwise
// the global active account (or env fallback).
func (r *ScopedResolver) Config(orgID, appID string) mailsmtp.Config {
	if appDB, ok := r.appDB(appID); ok {
		acc, err := appmail_query.NewAppMailAccountsQueryRepo(appDB, appID).GetActive()
		if err == nil && acc != nil {
			return toAppSMTPConfig(acc)
		}
		if err != nil && !errors.Is(err, appmail_query.ErrNoActive) && r.log != nil {
			r.log.Warning("scopedsettings: app %s: fetch active account failed: %v", appID, err)
		}
	}
	if orgDB, ok := r.orgDB(orgID); ok {
		acc, err := orgmail_query.NewOrgMailAccountsQueryRepo(orgDB).GetActive()
		if err == nil && acc != nil {
			return toSMTPConfig(acc)
		}
		if err != nil && !errors.Is(err, orgmail_query.ErrNoActive) && r.log != nil {
			r.log.Warning("scopedsettings: org %s: fetch active account failed: %v", orgID, err)
		}
	}
	return r.global.Config()
}

// TemplateForEvent returns the template name bound to event: the
// application's own binding wins if set (non-empty), else the org's
// own binding, else the global binding.
func (r *ScopedResolver) TemplateForEvent(orgID, appID, event string) string {
	if appDB, ok := r.appDB(appID); ok {
		if v, found, err := appmail_query.NewAppMailSettingsQueryRepo(appDB, appID).Get(appmail_query.EventTemplateKey(event)); err == nil && found && v != "" {
			return v
		}
	}
	if orgDB, ok := r.orgDB(orgID); ok {
		if v, found, err := orgmail_query.NewOrgMailSettingsQueryRepo(orgDB).Get(orgmail_query.EventTemplateKey(event)); err == nil && found && v != "" {
			return v
		}
	}
	return r.global.TemplateForEvent(event)
}

// AccountForEvent returns the SMTP account name bound to event, plus
// which tier it came from: resolvedAppID is non-empty when the
// application's own binding was used, resolvedOrgID is non-empty when
// the org's own binding was used (the two are mutually exclusive —
// exactly one, or neither, is set), and both empty means it fell
// through to the global binding. Callers MUST carry
// resolvedOrgID/resolvedAppID alongside the account name into the
// queue task — an account name is only meaningful within the tier it
// was resolved from (app, org, and global account names are separate
// namespaces), and the queue consumer needs to know where to look it
// up at send time, since account resolution can't happen synchronously
// across the queue boundary. See api/src/mail/consumer/consumer.go's
// org-aware selectMailer (app-aware wiring lands in step 5).
func (r *ScopedResolver) AccountForEvent(orgID, appID, event string) (name string, resolvedOrgID string, resolvedAppID string) {
	if appDB, ok := r.appDB(appID); ok {
		if v, found, err := appmail_query.NewAppMailSettingsQueryRepo(appDB, appID).Get(appmail_query.EventAccountKey(event)); err == nil && found && v != "" {
			return v, "", appID
		}
	}
	if orgDB, ok := r.orgDB(orgID); ok {
		if v, found, err := orgmail_query.NewOrgMailSettingsQueryRepo(orgDB).Get(orgmail_query.EventAccountKey(event)); err == nil && found && v != "" {
			return v, orgID, ""
		}
	}
	return r.global.AccountForEvent(event), "", ""
}

// RenderTemplate renders the application's own template named `name`
// if it exists and is active, else the org's own template of that
// name — ok is false (not an error) when neither tier has an override,
// signaling the caller to fall back to the existing global renderer
// (mail.MailService.Enqueue already does this whenever MailTask.Template
// is left set). Rendering happens here, synchronously, rather than
// deferred to the queue consumer — unlike SMTP accounts, a rendered
// subject/body is plain content, safe to carry across the queue
// boundary as-is (MailTask's existing TextBody/HTMLBody/Subject
// fields), so there's no need to re-resolve org/app context at consume
// time the way accounts require.
func (r *ScopedResolver) RenderTemplate(orgID, appID, name string, data map[string]interface{}) (subject, text, html string, ok bool, err error) {
	if appDB, found := r.appDB(appID); found {
		row, aerr := appmail_query.NewAppMailTemplatesQueryRepo(appDB, appID).GetByName(name)
		if aerr != nil && !errors.Is(aerr, appmail_query.ErrTemplateNotFound) {
			return "", "", "", false, aerr
		}
		if aerr == nil && row.IsActive {
			subject, text, html, err = mailtemplate.RenderStrings(name, row.Subject, row.TextBody, row.HTMLBody, data)
			if err != nil {
				return "", "", "", false, err
			}
			return subject, text, html, true, nil
		}
	}

	orgDB, found := r.orgDB(orgID)
	if !found {
		return "", "", "", false, nil
	}
	row, err := orgmail_query.NewOrgMailTemplatesQueryRepo(orgDB).GetByName(name)
	if err != nil {
		if errors.Is(err, orgmail_query.ErrTemplateNotFound) {
			return "", "", "", false, nil
		}
		return "", "", "", false, err
	}
	if !row.IsActive {
		return "", "", "", false, nil
	}
	subject, text, html, err = mailtemplate.RenderStrings(name, row.Subject, row.TextBody, row.HTMLBody, data)
	if err != nil {
		return "", "", "", false, err
	}
	return subject, text, html, true, nil
}

// ActivationSettings resolves the activation cadence: any application
// override (TTL/cooldown independently) wins over the org override,
// which wins over the global default.
func (r *ScopedResolver) ActivationSettings(orgID, appID string) mailsettings.ActivationSettings {
	out := r.global.Snapshot().Activation
	if orgDB, ok := r.orgDB(orgID); ok {
		settingsQuery := orgmail_query.NewOrgMailSettingsQueryRepo(orgDB)
		if v, found, err := settingsQuery.Get(orgmail_query.KeyActivationTTLHours); err == nil && found && v != "" {
			if n, perr := strconv.Atoi(v); perr == nil && n > 0 {
				out.TTLHours = n
			}
		}
		if v, found, err := settingsQuery.Get(orgmail_query.KeyActivationResendCooldown); err == nil && found && v != "" {
			if n, perr := strconv.Atoi(v); perr == nil && n >= 0 {
				out.ResendCooldownSeconds = n
			}
		}
	}
	if appDB, ok := r.appDB(appID); ok {
		settingsQuery := appmail_query.NewAppMailSettingsQueryRepo(appDB, appID)
		if v, found, err := settingsQuery.Get(appmail_query.KeyActivationTTLHours); err == nil && found && v != "" {
			if n, perr := strconv.Atoi(v); perr == nil && n > 0 {
				out.TTLHours = n
			}
		}
		if v, found, err := settingsQuery.Get(appmail_query.KeyActivationResendCooldown); err == nil && found && v != "" {
			if n, perr := strconv.Atoi(v); perr == nil && n >= 0 {
				out.ResendCooldownSeconds = n
			}
		}
	}
	return out
}

// orgDB opens the org's own users.db, or (false) if orgID is empty, no
// registry is configured, or the org can't be resolved — every caller
// treats a false return as "fall through", never an error.
func (r *ScopedResolver) orgDB(orgID string) (*sql.DB, bool) {
	if orgID == "" || r.orgReg == nil {
		return nil, false
	}
	db, err := orgrouter.ForOrg(r.orgReg, orgID)
	if err != nil {
		return nil, false
	}
	return db, true
}

// appDB opens the org users.db that owns appID (applications live
// inside their org's own users.db, scoped by application_id — there is
// no separate per-application database), or (false) if appID is empty,
// no registry is configured, or the application can't be found. Every
// caller treats a false return as "fall through to the org tier",
// never an error.
func (r *ScopedResolver) appDB(appID string) (*sql.DB, bool) {
	if appID == "" || r.orgReg == nil {
		return nil, false
	}
	db, _, err := orgrouter.OrgDBForApp(r.orgReg, appID)
	if err != nil {
		return nil, false
	}
	return db, true
}

func toSMTPConfig(a *orgmail_query.OrgMailAccount) mailsmtp.Config {
	return mailsmtp.Config{
		Host:     a.Host,
		Port:     a.Port,
		Username: a.Username,
		Password: a.Password,
		UseTLS:   a.UseTLS,
		From:     iam_mail.Address{Name: a.FromName, Email: a.FromEmail},
	}
}

func toAppSMTPConfig(a *appmail_query.AppMailAccount) mailsmtp.Config {
	return mailsmtp.Config{
		Host:     a.Host,
		Port:     a.Port,
		Username: a.Username,
		Password: a.Password,
		UseTLS:   a.UseTLS,
		From:     iam_mail.Address{Name: a.FromName, Email: a.FromEmail},
	}
}
