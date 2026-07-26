package entity

// SecurityStatus reports whether OS-level firewall enforcement is
// actually active, so the admin Attacks page can show a truthful
// warning instead of implying full network-level blocking when only
// the application-layer 429 path (section 1) is enforcing. See
// plan/ip-abuse-protection/plan.md sections 13-14.
type SecurityStatus struct {
	OS                string `json:"os" example:"linux"`
	FirewallBackend   string `json:"firewall_backend" example:"iptables"`
	FirewallAvailable bool   `json:"firewall_available" example:"true"`
	FirewallDetail    string `json:"firewall_detail,omitempty" example:"iptables not found in PATH (on Alpine: apk add iptables)"`
}

type SecurityStatusSuccess struct {
	Success bool           `json:"success" example:"true"`
	Message SecurityStatus `json:"message"`
}
