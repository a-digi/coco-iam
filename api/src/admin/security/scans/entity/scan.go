// Package entity holds the request/response types for the read-only
// admin port-scan-history API. See
// plan/port-scan-detection/plan.md Phase B.
package entity

// Scan is one port-scan episode from an IP — opened once the IP
// crosses the distinct-port threshold within the aggregation window,
// one row per episode (not one row per probed port). Lives in the
// same ip-attacks.db as ip_attacks, a separate table.
type Scan struct {
	ID            string `json:"id" example:"c3f6a9e2-1234-4a5b-8c9d-abcdef012345"`
	IP            string `json:"ip" example:"203.0.113.7"`
	StartedAt     string `json:"started_at" example:"2026-07-26T10:00:00Z"`
	LastSeenAt    string `json:"last_seen_at" example:"2026-07-26T10:04:12Z"`
	EndedAt       string `json:"ended_at,omitempty" example:"2026-07-26T10:30:00Z"`
	DistinctPorts int    `json:"distinct_ports" example:"14"`
	HitCount      int    `json:"hit_count" example:"37"`
	SamplePorts   string `json:"sample_ports" example:"22,80,443,3306,8080"`
	// GeoIPInfo is a JSON snapshot of the country/ASN/ISP resolved for
	// IP at the moment this episode opened — frozen then, never
	// re-derived later (geoip.db keeps no history of its own). Empty
	// if geoip was disabled, IP was loopback/private, or nothing
	// matched.
	GeoIPInfo string `json:"geoip_info,omitempty" example:"{\"country_code\":\"DE\",\"country\":\"Germany\",\"asn\":3320,\"as_org\":\"Deutsche Telekom AG\"}"`
}

// ScanListResponse is the list endpoint's payload — Total is the
// filtered row count (ignoring Limit/Offset), for the admin page's
// pagination.
type ScanListResponse struct {
	Scans  []Scan `json:"scans"`
	Total  int    `json:"total" example:"12"`
	Limit  int    `json:"limit" example:"50"`
	Offset int    `json:"offset" example:"0"`
}

// Swag-friendly success envelopes.

type ScanListSuccess struct {
	Success bool             `json:"success" example:"true"`
	Message ScanListResponse `json:"message"`
}

type ScanDetailSuccess struct {
	Success bool `json:"success" example:"true"`
	Message Scan `json:"message"`
}
