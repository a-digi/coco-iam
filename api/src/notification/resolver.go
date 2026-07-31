package notification

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"

	appmail_query "github.com/a-digi/coco-iam/src/applications/mail/repository/query"
	orgmail_query "github.com/a-digi/coco-iam/src/organizations/mail/repository/query"
	"github.com/a-digi/coco-iam/src/organizations/users/dbregistry"
	"github.com/a-digi/coco-iam/src/orgrouter"
	"github.com/a-digi/coco-logger/logger"
	coconotification "github.com/a-digi/coco-notification"
	"github.com/a-digi/coco-notification/mailer"
	notsettings "github.com/a-digi/coco-notification/settings"
	nottemplate "github.com/a-digi/coco-notification/template"
)

// ContextBagKey is where a single *ScopedResolver is stashed in the
// DI ContextBag.
const ContextBagKey = "notification.scoped_resolver"

// ScopedResolver wraps the generic notsettings.Resolver with
// organization- and application-level overrides. Resolution order:
// application → organization → global, falling through on
// empty/absent exactly like general.Store.BaseURL() and
// userrules.Store.GetForUser already do elsewhere in this codebase.
//
// It serves two roles:
//  1. The orgID/appID-shaped convenience API (Config/TemplateForEvent/
//     AccountForEvent/RenderTemplate/ActivationSettings) that
//     activation.Service, auth/recovery.Service, recoverypage.Service,
//     and organizations/users/notify.Service already call directly.
//  2. coconotification.SenderResolver — the seam the generic queue
//     consumer calls at send time to resolve a Task's SenderRef into
//     a concrete Sender, replacing this app's own hardcoded
//     app/org/global account-table branching that used to live
//     directly in mail/consumer.selectMailer.
type ScopedResolver struct {
	global         *notsettings.Resolver
	globalAccounts *mailer.Store
	orgReg         *dbregistry.OrgUserDBRegistry
	log            logger.Logger
}

// NewScopedResolver wires the org/app tiers on top of the existing
// global resolver. orgReg may be nil — every method degrades to the
// global resolver only, same as passing empty orgID/appID.
func NewScopedResolver(global *notsettings.Resolver, globalAccounts *mailer.Store, orgReg *dbregistry.OrgUserDBRegistry, log logger.Logger) *ScopedResolver {
	return &ScopedResolver{global: global, globalAccounts: globalAccounts, orgReg: orgReg, log: log}
}

// Config resolves the SMTP config to use: the application's own
// active account wins if set, otherwise the org's own active
// account, otherwise the global active account (or env fallback).
func (r *ScopedResolver) Config(orgID, appID string) mailer.Config {
	if appDB, ok := r.appDB(appID); ok {
		acc, err := appmail_query.NewAppMailAccountsQueryRepo(appDB, appID).GetActive()
		if err == nil && acc != nil {
			return toAppSMTPConfig(acc)
		}
		if err != nil && !errors.Is(err, appmail_query.ErrNoActive) && r.log != nil {
			r.log.Warning("notification: app %s: fetch active account failed: %v", appID, err)
		}
	}
	if orgDB, ok := r.orgDB(orgID); ok {
		acc, err := orgmail_query.NewOrgMailAccountsQueryRepo(orgDB).GetActive()
		if err == nil && acc != nil {
			return toOrgSMTPConfig(acc)
		}
		if err != nil && !errors.Is(err, orgmail_query.ErrNoActive) && r.log != nil {
			r.log.Warning("notification: org %s: fetch active account failed: %v", orgID, err)
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
// application's own binding was used, resolvedOrgID is non-empty
// when the org's own binding was used (the two are mutually
// exclusive), and both empty means it fell through to the global
// binding. Callers carry resolvedOrgID/resolvedAppID into the
// coconotification.Task's SenderRef.Scope so the queue consumer
// (via ResolveSender below) knows where to look the account name up
// at send time.
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
// name — ok is false (not an error) when neither tier has an
// override, signaling the caller to fall back to the existing
// global renderer.
func (r *ScopedResolver) RenderTemplate(orgID, appID, name string, data map[string]interface{}) (subject, text, html string, ok bool, err error) {
	if appDB, found := r.appDB(appID); found {
		row, aerr := appmail_query.NewAppMailTemplatesQueryRepo(appDB, appID).GetByName(name)
		if aerr != nil && !errors.Is(aerr, appmail_query.ErrTemplateNotFound) {
			return "", "", "", false, aerr
		}
		if aerr == nil && row.IsActive {
			subject, text, html, err = nottemplate.RenderStrings(name, row.Subject, row.TextBody, row.HTMLBody, data)
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
	subject, text, html, err = nottemplate.RenderStrings(name, row.Subject, row.TextBody, row.HTMLBody, data)
	if err != nil {
		return "", "", "", false, err
	}
	return subject, text, html, true, nil
}

// ActivationSettings resolves the activation cadence: any
// application override (TTL/cooldown independently) wins over the
// org override, which wins over the global default.
func (r *ScopedResolver) ActivationSettings(orgID, appID string) notsettings.ActivationSettings {
	out := r.global.ActivationSettings()
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

// ResolveSender implements coconotification.SenderResolver — called
// by the generic queue consumer whenever a Task's SenderRef.Name is
// non-empty. AppID/OrgID scope keys decide WHERE Name is looked up,
// and are mutually exclusive: Scope["app_id"] non-empty means that
// application's own accounts table (application_id-scoped, inside
// its owning org's users.db — applications have no database of
// their own); Scope["org_id"] non-empty means that organization's own
// accounts table; neither means the GLOBAL mailer_smtp_accounts
// table. All three are completely separate namespaces, never
// cross-checked against each other, so a same-named account at one
// tier can never be mistaken for another tier's account.
func (r *ScopedResolver) ResolveSender(_ context.Context, ref coconotification.SenderRef) (coconotification.Sender, error) {
	if appID := ref.Scope["app_id"]; appID != "" {
		if r.orgReg == nil {
			return nil, fmt.Errorf("notification: task references app %q account %q but no org registry is configured", appID, ref.Name)
		}
		appDB, _, err := orgrouter.OrgDBForApp(r.orgReg, appID)
		if err != nil {
			return nil, fmt.Errorf("notification: app %q lookup failed: %w", appID, err)
		}
		acc, err := appmail_query.NewAppMailAccountsQueryRepo(appDB, appID).GetByName(ref.Name)
		if err != nil {
			return nil, fmt.Errorf("notification: app %q account %q lookup failed: %w", appID, ref.Name, err)
		}
		return mailer.New(toAppSMTPConfig(acc), r.log), nil
	}

	if orgID := ref.Scope["org_id"]; orgID != "" {
		if r.orgReg == nil {
			return nil, fmt.Errorf("notification: task references org %q account %q but no org registry is configured", orgID, ref.Name)
		}
		orgDB, err := orgrouter.ForOrg(r.orgReg, orgID)
		if err != nil {
			return nil, fmt.Errorf("notification: org %q lookup failed: %w", orgID, err)
		}
		acc, err := orgmail_query.NewOrgMailAccountsQueryRepo(orgDB).GetByName(ref.Name)
		if err != nil {
			return nil, fmt.Errorf("notification: org %q account %q lookup failed: %w", orgID, ref.Name, err)
		}
		return mailer.New(toOrgSMTPConfig(acc), r.log), nil
	}

	if r.globalAccounts == nil {
		return nil, fmt.Errorf("notification: task references account %q but no global accounts store is configured", ref.Name)
	}
	acc, err := r.globalAccounts.GetByName(ref.Name)
	if err != nil {
		return nil, fmt.Errorf("notification: account %q lookup failed: %w", ref.Name, err)
	}
	return mailer.New(mailer.Config{
		Host: acc.Host, Port: acc.Port, Username: acc.Username, Password: acc.Password, UseTLS: acc.UseTLS,
		From: coconotification.Address{Name: acc.FromName, Email: acc.FromEmail},
	}, r.log), nil
}

// orgDB opens the org's own users.db, or (false) if orgID is empty,
// no registry is configured, or the org can't be resolved — every
// caller treats a false return as "fall through", never an error.
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
// inside their org's own users.db, scoped by application_id — there
// is no separate per-application database), or (false) if appID is
// empty, no registry is configured, or the application can't be
// found.
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

func toOrgSMTPConfig(a *orgmail_query.OrgMailAccount) mailer.Config {
	return mailer.Config{
		Host:     a.Host,
		Port:     a.Port,
		Username: a.Username,
		Password: a.Password,
		UseTLS:   a.UseTLS,
		From:     coconotification.Address{Name: a.FromName, Email: a.FromEmail},
	}
}

func toAppSMTPConfig(a *appmail_query.AppMailAccount) mailer.Config {
	return mailer.Config{
		Host:     a.Host,
		Port:     a.Port,
		Username: a.Username,
		Password: a.Password,
		UseTLS:   a.UseTLS,
		From:     coconotification.Address{Name: a.FromName, Email: a.FromEmail},
	}
}
