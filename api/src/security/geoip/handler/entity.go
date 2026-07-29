// Package handler serves the admin GeoIP settings + process-control
// API under /api/v1/admin/security/geoip/*. Deliberately lives inside
// api/src/security/geoip rather than following the
// api/src/admin/security/{attacks,scans}/handler convention used
// elsewhere in this plan — an explicit, repeated instruction, not an
// oversight. See plan/geoip-enrichment/plan.md's "Admin-editable
// settings" section.
package handler

// licenseKeyMask is returned in place of the real MaxMind license key
// on every read path — never the raw secret. Re-implemented here as a
// local literal rather than importing
// api/src/auth/crypto/secretbox.MaskSecret() (which produces the same
// string), per the "no shared code" instruction for this feature.
const licenseKeyMask = "••••••••"

// SettingsResponse is the GET/PUT response shape for geoip settings.
// MaxMindLicenseKeyMask is populated (to licenseKeyMask) only when a
// key is actually stored — the raw key itself is never returned.
type SettingsResponse struct {
	Enabled               bool   `json:"enabled" example:"true"`
	MaxMindAccountID      string `json:"maxmind_account_id" example:"123456"`
	MaxMindLicenseKeyMask string `json:"maxmind_license_key_mask,omitempty" example:"••••••••"`
	CheckIntervalSeconds  int    `json:"check_interval_seconds" example:"600"`
	PullIntervalHours     int    `json:"pull_interval_hours" example:"24"`
}

// SettingsRequest is the PUT request body. An empty/omitted
// MaxMindLicenseKey means "leave the currently-stored key
// unchanged" — standard admin-UI secret-field UX (the field only
// ever displays a mask, never the real value, so there's nothing
// meaningful to submit back unless the admin is actually changing
// it). Enforced at the repository layer
// (geoip.SettingsPersistentRepo.SaveSettings), not here, so the
// invariant holds regardless of caller.
type SettingsRequest struct {
	Enabled              bool   `json:"enabled" example:"true"`
	MaxMindAccountID     string `json:"maxmind_account_id" example:"123456"`
	MaxMindLicenseKey    string `json:"maxmind_license_key,omitempty" example:"a1b2c3d4e5f6"`
	CheckIntervalSeconds int    `json:"check_interval_seconds" example:"600"`
	PullIntervalHours    int    `json:"pull_interval_hours" example:"24"`
}

// StatusResponse reports the geoip-updater process's current state.
// PID and LastPulledAt are omitted (zero value) when not applicable —
// PID when not running, LastPulledAt when geoip.db has never been
// successfully populated. CityRangeCount/ASNRangeCount are 0 in that
// same "never populated yet" case.
type StatusResponse struct {
	Running        bool   `json:"running" example:"true"`
	PID            int    `json:"pid,omitempty" example:"48213"`
	Enabled        bool   `json:"enabled" example:"true"`
	LastPulledAt   string `json:"last_pulled_at,omitempty" example:"2026-07-29T09:00:00Z"`
	CityRangeCount int    `json:"city_range_count" example:"1108580"`
	ASNRangeCount  int    `json:"asn_range_count" example:"573104"`
}

// Swag-friendly success envelopes.

type SettingsSuccess struct {
	Success bool             `json:"success" example:"true"`
	Message SettingsResponse `json:"message"`
}

type StatusSuccess struct {
	Success bool           `json:"success" example:"true"`
	Message StatusResponse `json:"message"`
}
