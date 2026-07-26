package entity

// SecurityStatus reports whether OS-level firewall enforcement is
// actually active, so the admin Attacks page can show a truthful
// warning instead of implying full network-level blocking when only
// the application-layer 429 path (section 1) is enforcing. See
// plan/ip-abuse-protection/plan.md sections 13-14.
//
// It also reports ip-attacks.db's current size relative to the
// archiving threshold, and how many prior generations have already
// been rotated out — see plan/ip-attacks-db-archiving/plan.md.
type SecurityStatus struct {
	OS                string `json:"os" example:"linux"`
	FirewallBackend   string `json:"firewall_backend" example:"iptables"`
	FirewallAvailable bool   `json:"firewall_available" example:"true"`
	FirewallDetail    string `json:"firewall_detail,omitempty" example:"iptables not found in PATH (on Alpine: apk add iptables)"`

	IPAttacksEntryCount    int64 `json:"ip_attacks_entry_count" example:"1245"`
	IPAttacksThreshold     int64 `json:"ip_attacks_threshold" example:"100000000"`
	IPAttacksArchivesCount int   `json:"ip_attacks_archives_count" example:"2"`

	// ScanWatch* report whether port-scan detection actually has a log
	// source available on this host — see
	// plan/port-scan-detection/plan.md Phase B. When
	// ScanWatchAvailable is false, no port-scan visibility exists at
	// all (not even the application-layer visibility ip_attacks has),
	// since there is nothing to ingest.
	ScanWatchSource    string `json:"scan_watch_source" example:"journald"`
	ScanWatchAvailable bool   `json:"scan_watch_available" example:"true"`
	ScanWatchDetail    string `json:"scan_watch_detail,omitempty" example:"neither journalctl nor a readable syslog file (/var/log/messages) was found"`
}

type SecurityStatusSuccess struct {
	Success bool           `json:"success" example:"true"`
	Message SecurityStatus `json:"message"`
}
