// Package entity holds the request/response types for the
// per-application end-user login-attempt audit log. See
// plan/login-audit-log/plan.md.
package entity

// ApplicationLoginAttempt is one application end-user login attempt
// (success or failure) against /applications/authenticate. Lives in
// that application's own <slug>_login.db, not the org's users.db.
// Passwords are never stored here.
type ApplicationLoginAttempt struct {
	ID string `json:"id" example:"b1f6c9e2-1234-4a5b-8c9d-abcdef012345"`
	// ApplicationUserID is empty when the typed username never
	// resolved to a real end-user account.
	ApplicationUserID string `json:"application_user_id,omitempty" example:"a3bead61-9f22-ff1f-8315-117b1ce373d0"`
	Username          string `json:"username" example:"jdoe"`
	Success           bool   `json:"success" example:"false"`
	// FailureReason is one of invalid_credentials, inactive_user, or
	// empty on success. inactive_user covers the application-level
	// password-login-disabled rejection, not just an inactive user
	// account.
	FailureReason string `json:"failure_reason,omitempty" example:"invalid_credentials"`
	IP            string `json:"ip" example:"203.0.113.7"`
	UserAgent     string `json:"user_agent,omitempty" example:"Mozilla/5.0 ..."`
	CreatedAt     string `json:"created_at" example:"2026-07-29T20:41:00Z"`
	// GeoIPInfo is a JSON snapshot of the country/city/ISP resolved
	// for ip at record time — empty when the IP was loopback/private,
	// GeoIP had no coverage, GeoIP is disabled, or the row predates
	// this field. Never re-derived later. See
	// plan/login-log-geoip/plan.md.
	GeoIPInfo string `json:"geoip_info,omitempty" example:"{\"country_code\":\"DE\",\"country\":\"Germany\",\"city\":\"Berlin\",\"asn\":3320,\"as_org\":\"Deutsche Telekom AG\"}"`
}

// ApplicationLoginAttemptListResponse is the list endpoint's payload —
// Total is the filtered row count (ignoring Limit/Offset), for the
// admin page's pagination.
type ApplicationLoginAttemptListResponse struct {
	Attempts []ApplicationLoginAttempt `json:"attempts"`
	Total    int                       `json:"total" example:"128"`
	Limit    int                       `json:"limit" example:"50"`
	Offset   int                       `json:"offset" example:"0"`
}

// Swag-friendly success envelope.

type ApplicationLoginAttemptListSuccess struct {
	Success bool                                `json:"success" example:"true"`
	Message ApplicationLoginAttemptListResponse `json:"message"`
}
