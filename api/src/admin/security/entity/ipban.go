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

// Swag-friendly success envelopes.

type IPBanListSuccess struct {
	Success bool    `json:"success" example:"true"`
	Message []IPBan `json:"message"`
}

type IPBanSuccess struct {
	Success bool  `json:"success" example:"true"`
	Message IPBan `json:"message"`
}

type StatusSuccess struct {
	Success bool           `json:"success" example:"true"`
	Message StatusResponse `json:"message"`
}
