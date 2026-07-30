package entity

// FirewallResyncResponse reports the outcome of re-applying every
// currently-active (non-expired) ip_bans row through the same Ban()
// path a fresh ban already uses — useful after a host reboot resets
// pf's runtime state, or an admin manually flushed the OS firewall.
// See plan/firewall-page/plan.md.
type FirewallResyncResponse struct {
	Synced         int `json:"synced" example:"3"`
	SkippedExpired int `json:"skipped_expired" example:"1"`
	Failed         int `json:"failed" example:"0"`
}

// Swag-friendly success envelope.

type FirewallResyncSuccess struct {
	Success bool                   `json:"success" example:"true"`
	Message FirewallResyncResponse `json:"message"`
}

// FirewallRulesResponse lists the IPs currently blocked at the OS
// firewall level, read live from the backend in use — informational
// only; ip_bans (via /admin/security/ip-bans) stays the source of
// truth for what *should* be banned. See plan/firewall-live-rules/plan.md.
type FirewallRulesResponse struct {
	Backend string   `json:"backend" example:"iptables"`
	Rules   []string `json:"rules"`
}

// Swag-friendly success envelope.

type FirewallRulesSuccess struct {
	Success bool                  `json:"success" example:"true"`
	Message FirewallRulesResponse `json:"message"`
}
