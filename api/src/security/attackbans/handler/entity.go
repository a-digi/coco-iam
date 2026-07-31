// Package handler serves the admin attack ban-rule settings API under
// /api/v1/admin/security/attack-bans/*. Deliberately lives inside
// api/src/security/attackbans, mirroring where
// api/src/security/loginbans/handler already put its own settings
// handlers. See plan/attack-ban-rules/plan.md.
package handler

// SettingsResponse is the GET/PUT response shape — a single global
// rule, not split by domain like loginbans' admin/application.
type SettingsResponse struct {
	Enabled       bool `json:"enabled" example:"true"`
	Threshold     int  `json:"threshold" example:"50"`
	WindowSeconds int  `json:"window_seconds" example:"60"`
	BanSeconds    int  `json:"ban_seconds" example:"3600"`
}

// SettingsRequest is the PUT request body.
type SettingsRequest struct {
	Enabled       bool `json:"enabled" example:"true"`
	Threshold     int  `json:"threshold" example:"50"`
	WindowSeconds int  `json:"window_seconds" example:"60"`
	BanSeconds    int  `json:"ban_seconds" example:"3600"`
}

// Swag-friendly success envelope.

type SettingsSuccess struct {
	Success bool             `json:"success" example:"true"`
	Message SettingsResponse `json:"message"`
}
