// Package settings stores event→template bindings for system mail events
// in mail.db. SMTP connection settings now live in the `accounts` package
// as first-class rows; this package keeps the event-binding KV plus the
// legacy SMTP key constants used by the one-shot migration shim.
package settings

import "github.com/a-digi/coco-iam/src/mail/accounts"

// Legacy SMTP keys — read (and deleted) once by the migration shim, then
// never again. New writes never target these keys.
const (
	KeySMTPHost      = "smtp.host"
	KeySMTPPort      = "smtp.port"
	KeySMTPUsername  = "smtp.username"
	KeySMTPPassword  = "smtp.password"
	KeySMTPFromName  = "smtp.from_name"
	KeySMTPFromEmail = "smtp.from_email"
	KeySMTPTLS       = "smtp.tls"
)

// EventTemplateKey returns the settings key that stores the template name
// bound to the given event.
func EventTemplateKey(event string) string {
	return "event." + event + ".template"
}

// EventAccountKey returns the settings key that stores the SMTP account
// name bound to the given event. Accounts are addressed by name (not id)
// so a DB reseed with the same name preserves the binding.
func EventAccountKey(event string) string {
	return "event." + event + ".account"
}

// Event describes one system event the UI exposes as a template binding.
type Event struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Description string `json:"description"`
}

// EventCatalog is the authoritative list of template-bindable events.
var EventCatalog = []Event{
	{
		Key:         "admin_invite",
		Label:       "Admin invitation",
		Description: "Sent when an administrator is invited to the system.",
	},
	{
		Key:         "user_invite",
		Label:       "User invitation",
		Description: "Sent when a regular (non-admin) user is created and needs to activate their account.",
	},
	{
		Key:         "password_recovery",
		Label:       "Password recovery",
		Description: "Sent when a user requests a password reset link.",
	},
	{
		Key:         "admin_password_expiry_warning",
		Label:       "Admin password expiry warning",
		Description: "Sent to an administrator whose password is about to expire.",
	},
	{
		Key:         "user_password_expiry_warning",
		Label:       "User password expiry warning",
		Description: "Sent to an organisation user whose password is about to expire.",
	},
	{
		Key:         "org_user_deactivated",
		Label:       "Organisation user deactivated",
		Description: "Sent to an organisation user when their account is deactivated.",
	},
	{
		Key:         "org_user_removed",
		Label:       "Organisation user removed",
		Description: "Sent to an organisation user when their account is permanently removed.",
	},
}

// EventBinding pairs an event key with the template name and SMTP account
// name used when that system event fires. Template and account are
// required as a pair on save; both empty means the binding is unset.
type EventBinding struct {
	Event    string `json:"event"`
	Template string `json:"template"`
	Account  string `json:"account"`
}

// Activation cadence keys stored in mail_settings. The frontend base URL
// has moved to the general app_settings store — it's no longer
// mail-specific. TTL and resend cooldown stay here because they control
// the activation email flow itself.
const (
	KeyActivationTTLHours       = "activation.ttl_hours"
	KeyActivationResendCooldown = "activation.resend_cooldown_seconds"
)

// ActivationSettings exposes the cadence knobs the UI edits on the Email
// settings page. Base URL is not part of this payload — see
// /admin/settings/general for it.
type ActivationSettings struct {
	TTLHours              int `json:"ttl_hours"`
	ResendCooldownSeconds int `json:"resend_cooldown_seconds"`
}

// Snapshot is the admin GET /admin/mail/settings response shape.
// ActiveAccount is nil if no account is currently active — the UI shows
// a "configure an account" CTA in that case.
type Snapshot struct {
	ActiveAccount *accounts.Account  `json:"active_account"`
	Events        []EventBinding     `json:"events"`
	Activation    ActivationSettings `json:"activation"`
}
