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

// FirewallRuleEntry is one distinct IP currently blocked at the OS
// firewall level, with how many underlying rules exist for it — a
// count above 1 means duplicated rules (Ban() never checks for an
// existing rule before inserting another; see
// plan/firewall-live-rules/plan.md's follow-up on this feature).
type FirewallRuleEntry struct {
	IP    string `json:"ip" example:"203.0.113.7"`
	Count int    `json:"count" example:"3"`
}

// FirewallRulesResponse lists the IPs currently blocked at the OS
// firewall level, read live from the backend in use — informational
// only; ip_bans (via /admin/security/ip-bans) stays the source of
// truth for what *should* be banned. See plan/firewall-live-rules/plan.md.
type FirewallRulesResponse struct {
	Backend string              `json:"backend" example:"iptables"`
	Rules   []FirewallRuleEntry `json:"rules"`
}

// Swag-friendly success envelope.

type FirewallRulesSuccess struct {
	Success bool                  `json:"success" example:"true"`
	Message FirewallRulesResponse `json:"message"`
}

// FirewallRuleRemoveResponse reports the outcome of manually removing
// every OS-level rule for one IP. Resynced is true when the IP was
// still actively banned in ip_bans, in which case exactly one clean
// rule was immediately re-applied — see
// IPGuardSecurityLayer.RemoveAllFirewallRules' doc comment for why.
type FirewallRuleRemoveResponse struct {
	Removed  int  `json:"removed" example:"3"`
	Resynced bool `json:"resynced" example:"false"`
}

// Swag-friendly success envelope.

type FirewallRuleRemoveSuccess struct {
	Success bool                       `json:"success" example:"true"`
	Message FirewallRuleRemoveResponse `json:"message"`
}
