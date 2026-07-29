// Package handler serves the admin failed-login ban-rule settings API
// under /api/v1/admin/security/login-bans/*. Deliberately lives
// inside api/src/security/loginbans rather than
// api/src/admin/security/{attacks,scans}/handler, mirroring where
// api/src/security/geoip/handler already put its own settings
// handlers. See plan/login-ban-rules/plan.md.
package handler

// DomainRuleResponse is one domain's (admin or application) ban-rule
// settings, both on GET responses and PUT request bodies.
type DomainRuleResponse struct {
	Enabled       bool `json:"enabled" example:"true"`
	Threshold     int  `json:"threshold" example:"5"`
	WindowSeconds int  `json:"window_seconds" example:"600"`
	BanSeconds    int  `json:"ban_seconds" example:"3600"`
}

// SettingsResponse is the GET/PUT response shape.
type SettingsResponse struct {
	Admin       DomainRuleResponse `json:"admin"`
	Application DomainRuleResponse `json:"application"`
}

// SettingsRequest is the PUT request body.
type SettingsRequest struct {
	Admin       DomainRuleResponse `json:"admin"`
	Application DomainRuleResponse `json:"application"`
}

// Swag-friendly success envelope.

type SettingsSuccess struct {
	Success bool             `json:"success" example:"true"`
	Message SettingsResponse `json:"message"`
}
