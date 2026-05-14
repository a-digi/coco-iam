package mail

import "context"

// Mailer is the low-level interface for sending a single, fully-rendered
// message to an underlying transport (SMTP today, something else tomorrow).
//
// Only the queue consumer should hold a direct Mailer reference — application
// code goes through MailService so retries / DLQ / templating are consistent.
type Mailer interface {
	Send(ctx context.Context, msg Message) error
}

// ContextBagKey values used when registering the concrete implementations
// in config/di. Keeping them here (instead of in config/di) avoids an import
// cycle between the mail package and the DI container.
const (
	ContextBagKeyMailer             = "mail.Mailer"
	ContextBagKeyMailService        = "mail.MailService"
	ContextBagKeyMailStore          = "mail.Store"
	ContextBagKeyTemplateRepository = "mail.TemplateRepository"
	ContextBagKeySettingsStore      = "mail.SettingsStore"
	ContextBagKeySettingsResolver   = "mail.SettingsResolver"
	ContextBagKeyAccountsStore      = "mail.AccountsStore"
)

// QueueNameOutbound is the queue name used for outbound mail delivery.
const QueueNameOutbound = "mail-outbound"
