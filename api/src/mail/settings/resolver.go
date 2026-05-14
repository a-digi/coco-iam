package settings

import (
	"errors"
	"strconv"

	iam_mail "github.com/a-digi/coco-iam/src/mail"
	"github.com/a-digi/coco-iam/src/mail/accounts"
	mailsmtp "github.com/a-digi/coco-iam/src/mail/smtp"
	"github.com/a-digi/coco-logger/logger"
)

// Resolver produces the live SMTP config and event→template bindings used
// by the mail engine. SMTP now comes from the active row in
// `mail_smtp_accounts`; when no active account exists, the resolver falls
// back to the boot-time env config so a fresh install still works.
type Resolver struct {
	store    *Store            // event-binding KV
	accounts *accounts.Store   // SMTP accounts — one is active
	envCfg   mailsmtp.Config   // env fallback
	log      logger.Logger
}

// NewResolver wires the resolver. `envCfg` is parsed from environment
// variables at boot time; it's used only when no active account exists.
func NewResolver(store *Store, accts *accounts.Store, envCfg mailsmtp.Config, log logger.Logger) *Resolver {
	return &Resolver{store: store, accounts: accts, envCfg: envCfg, log: log}
}

// Config returns the SMTP config in effect right now. Active account wins;
// env fills the gap otherwise.
func (r *Resolver) Config() mailsmtp.Config {
	if r.accounts != nil {
		acc, err := r.accounts.GetActive()
		if err == nil && acc != nil {
			return toSMTPConfig(acc)
		}
		if err != nil && !errors.Is(err, accounts.ErrNoActive) {
			r.log.Warning("mail resolver: fetch active account failed: %v", err)
		}
	}
	return r.envCfg
}

// ActiveAccount returns the currently active account (or nil) for the
// admin GET endpoint. Caller is responsible for redacting the password.
func (r *Resolver) ActiveAccount() *accounts.Account {
	if r.accounts == nil {
		return nil
	}
	acc, err := r.accounts.GetActive()
	if err != nil {
		return nil
	}
	return acc
}

// TemplateForEvent returns the template name bound to the given event
// key, or "" if none is configured.
func (r *Resolver) TemplateForEvent(event string) string {
	v, ok, _ := r.store.Get(EventTemplateKey(event))
	if !ok {
		return ""
	}
	return v
}

// AccountForEvent returns the SMTP account name bound to the given event,
// or "" if none is configured. Callers combine this with TemplateForEvent
// to resolve a full event → (template, account) binding.
func (r *Resolver) AccountForEvent(event string) string {
	v, ok, _ := r.store.Get(EventAccountKey(event))
	if !ok {
		return ""
	}
	return v
}

// Snapshot returns the current view for the admin GET endpoint. Handlers
// redact the account password before serialisation.
func (r *Resolver) Snapshot() Snapshot {
	out := Snapshot{
		ActiveAccount: r.ActiveAccount(),
		Events:        make([]EventBinding, 0, len(EventCatalog)),
		Activation:    r.activationSettings(),
	}
	for _, evt := range EventCatalog {
		tpl, _, _ := r.store.Get(EventTemplateKey(evt.Key))
		acc, _, _ := r.store.Get(EventAccountKey(evt.Key))
		out.Events = append(out.Events, EventBinding{Event: evt.Key, Template: tpl, Account: acc})
	}
	return out
}

// activationSettings reads the two cadence knobs with sensible defaults
// so the UI always has a value to bind to. The frontend base URL is no
// longer part of this snapshot — it moved to general.Store.
func (r *Resolver) activationSettings() ActivationSettings {
	out := ActivationSettings{
		TTLHours:              24,
		ResendCooldownSeconds: 300,
	}
	if v, ok, _ := r.store.Get(KeyActivationTTLHours); ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			out.TTLHours = n
		}
	}
	if v, ok, _ := r.store.Get(KeyActivationResendCooldown); ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			out.ResendCooldownSeconds = n
		}
	}
	return out
}

// toSMTPConfig converts an accounts.Account to the smtp.Config shape the
// mailer expects.
func toSMTPConfig(a *accounts.Account) mailsmtp.Config {
	return mailsmtp.Config{
		Host:     a.Host,
		Port:     a.Port,
		Username: a.Username,
		Password: a.Password,
		UseTLS:   a.UseTLS,
		From: iam_mail.Address{
			Name:  a.FromName,
			Email: a.FromEmail,
		},
	}
}
