// Package admin hosts the HTTP handlers for general app settings.
// Per-org branding (base URL, page title, description, robots) is managed
// via OrgGeneralSettingsGetHandler / OrgGeneralSettingsUpdateHandler.
// The legacy global /settings/general endpoints are no longer active.
package admin


type bagGetter interface {
	Get(key string) (interface{}, bool)
}
