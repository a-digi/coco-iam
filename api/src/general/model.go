// Package general stores app-wide settings (branding, public URLs, SEO
// metadata) in the main users.db. These values are agnostic to email,
// queues, or any other subsystem — the mail engine, activation service,
// and any future link-building code read them from here rather than
// carrying their own copy.
package general

// Key constants for the app_settings KV table. Grouped by concern; every
// key is namespaced under `general.` so future top-level namespaces
// (feature flags etc.) can coexist in the same table.
const (
	KeyBaseURL     = "general.base_url"     // absolute URL of the public frontend
	KeyPageTitle   = "general.page_title"   // product / instance name shown in UI + emails
	KeyDescription = "general.description"  // short marketing blurb; <meta name="description">
	KeyRobots      = "general.robots"       // <meta name="robots"> content, e.g. "index, follow"
)

// ContextBagKeyStore is where main.go stashes the Store for handlers that
// need it (activation service, admin/public settings handlers).
const ContextBagKeyStore = "general.Store"

// Settings is the shape returned by both the admin GET and the public
// GET — the public endpoint returns the same JSON without auth.
type Settings struct {
	BaseURL     string `json:"base_url"`
	PageTitle   string `json:"page_title"`
	Description string `json:"description"`
	Robots      string `json:"robots"`
}
