package notification

// ContextBagKey* values used when wiring the concrete implementations
// in main.go. Kept here (rather than in config/di) to avoid an
// import cycle, mirroring the old api/src/mail package's own
// convention.
const (
	ContextBagKeySender             = "notification.Sender"
	ContextBagKeyService            = "notification.Service"
	ContextBagKeyStore              = "notification.Store"
	ContextBagKeyTemplateRepository = "notification.TemplateRepository"
	ContextBagKeySettingsStore      = "notification.SettingsStore"
	ContextBagKeySettingsResolver   = "notification.SettingsResolver"
	ContextBagKeyAccountsStore      = "notification.AccountsStore"
	ContextBagKeyOrchestrator       = "notification.Orchestrator"
)
