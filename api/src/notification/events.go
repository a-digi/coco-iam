// Package notification is coco-iam's own wiring around the generic
// github.com/a-digi/coco-notification library: the domain event
// catalog (which events this app fires), the DI ContextBagKey
// convention this app's boot code uses, and ScopedResolver — the
// concrete coconotification.SenderResolver implementation that knows
// how to cascade application → organization → global across this
// app's own multi-tenant SQLite-per-org schema. See
// plan/org-app-email-settings/plan.md and
// plan/coco-notification-extraction/plan.md.
package notification

import notsettings "github.com/a-digi/coco-notification/settings"

// EventCatalog is the authoritative list of template-bindable
// system events for this application. Moved verbatim from the old
// api/src/mail/settings.EventCatalog — the generic
// coco-notification/settings package deliberately doesn't ship one,
// since which events exist is a domain concept only the calling
// application knows.
var EventCatalog = []notsettings.Event{
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
	{
		Key:         "app_registration_notification",
		Label:       "Application registration notification",
		Description: "Sent to an existing user who has just been mapped to a new application via self-registration.",
	},
}
