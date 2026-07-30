// Package entity holds the request/response types for the admin IP
// ban/allowlist management API. See plan/ip-abuse-protection/plan.md.
package entity

// IPBan is a currently-active (or not-yet-pruned expired) rate-limit
// ban on an IP address.
type IPBan struct {
	IP        string `json:"ip" example:"203.0.113.7"`
	Tier      string `json:"tier" example:"sensitive"`
	Reason    string `json:"reason" example:"sensitive rate limit exceeded"`
	BannedAt  string `json:"banned_at" example:"2026-07-26T10:00:00Z"`
	ExpiresAt string `json:"expires_at" example:"2026-07-26T11:00:00Z"`
	HitCount  int    `json:"hit_count" example:"3"`
	CreatedBy string `json:"created_by,omitempty" example:"6b12ba0b-6b36-4a94-bce4-6ba3615b1f85"`
}

// IPBanRequest is the body for manually banning an IP.
type IPBanRequest struct {
	IP              string `json:"ip" example:"203.0.113.7"`
	Reason          string `json:"reason" example:"manually banned by admin"`
	DurationMinutes int    `json:"duration_minutes" example:"60"`
}

// StatusResponse is a minimal {status: "..."} payload for actions that
// don't otherwise return a resource (e.g. unbanning an IP).
type StatusResponse struct {
	Status string `json:"status" example:"unbanned"`
}

// FailedUsernameSummary is one username a banned IP attempted (admin
// console or one application, depending on which section it appears
// under), aggregated across every failed attempt ever recorded from
// that IP — not scoped to the specific window that triggered any one
// ban, since that window may since have changed or the ban may be
// manual. See plan/ip-ban-accounts/plan.md.
type FailedUsernameSummary struct {
	Username      string `json:"username" example:"jdoe"`
	AccountID     string `json:"account_id,omitempty" example:"793424dd-7913-4190-be88-b928559ab4ef"`
	Attempts      int    `json:"attempts" example:"3"`
	LastAttemptAt string `json:"last_attempt_at" example:"2026-07-30T20:41:00Z"`
}

// ApplicationFailedUsernameSummary is one username a banned IP tried
// against one specific application — same shape as
// FailedUsernameSummary plus which application, since a single IP's
// application-side attempts can span multiple applications.
type ApplicationFailedUsernameSummary struct {
	ApplicationID    string `json:"application_id" example:"5543a098-4e99-4778-859e-ab54328f47d6"`
	ApplicationTitle string `json:"application_title" example:"Login Flow Test App"`
	Username         string `json:"username" example:"jdoe"`
	AccountID        string `json:"account_id,omitempty" example:"a3bead61-9f22-ff1f-8315-117b1ce373d0"`
	Attempts         int    `json:"attempts" example:"3"`
	LastAttemptAt    string `json:"last_attempt_at" example:"2026-07-30T20:41:00Z"`
}

// IPBanAccountsResponse summarizes which accounts a banned IP tried.
// AdminAttempts is nil when the caller lacks admin:security:login-log:read;
// ApplicationAttempts is nil when the caller lacks
// applications:login_log:read — both independent of the
// admin:security:ipbans:read scope gating this endpoint itself, and
// each never an empty slice when unauthorized, so the frontend can
// distinguish "not authorized to see this" from "authorized, found
// nothing". See plan/ip-ban-accounts/plan.md.
type IPBanAccountsResponse struct {
	AdminAttempts       []FailedUsernameSummary            `json:"admin_attempts"`
	ApplicationAttempts []ApplicationFailedUsernameSummary `json:"application_attempts"`
}

// Swag-friendly success envelopes.

type IPBanListSuccess struct {
	Success bool    `json:"success" example:"true"`
	Message []IPBan `json:"message"`
}

type IPBanSuccess struct {
	Success bool  `json:"success" example:"true"`
	Message IPBan `json:"message"`
}

type IPBanAccountsSuccess struct {
	Success bool                  `json:"success" example:"true"`
	Message IPBanAccountsResponse `json:"message"`
}

type StatusSuccess struct {
	Success bool           `json:"success" example:"true"`
	Message StatusResponse `json:"message"`
}
