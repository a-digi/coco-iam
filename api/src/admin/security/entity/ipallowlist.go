package entity

// IPAllowlistEntry is an IP address exempted from rate limiting/bans
// entirely — the relief valve for legitimate shared-IP traffic (NAT/
// office egress) that would otherwise trip the global tier.
type IPAllowlistEntry struct {
	IP        string `json:"ip" example:"203.0.113.7"`
	Note      string `json:"note,omitempty" example:"office egress"`
	CreatedAt string `json:"created_at" example:"2026-07-26T10:00:00Z"`
	CreatedBy string `json:"created_by" example:"6b12ba0b-6b36-4a94-bce4-6ba3615b1f85"`
}

// IPAllowlistRequest is the body for adding an IP to the allowlist.
type IPAllowlistRequest struct {
	IP   string `json:"ip" example:"203.0.113.7"`
	Note string `json:"note,omitempty" example:"office egress"`
}

// Swag-friendly success envelopes.

type IPAllowlistListSuccess struct {
	Success bool               `json:"success" example:"true"`
	Message []IPAllowlistEntry `json:"message"`
}

type IPAllowlistEntrySuccess struct {
	Success bool             `json:"success" example:"true"`
	Message IPAllowlistEntry `json:"message"`
}
