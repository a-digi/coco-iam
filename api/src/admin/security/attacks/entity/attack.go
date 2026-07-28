// Package entity holds the request/response types for the read-only
// admin attack-history API. See plan/ip-abuse-protection/plan.md
// sections 10 and 13.
package entity

// Attack is one historical (or currently ongoing, if EndedAt is
// empty) IP-abuse episode — one row per distinct episode, not one row
// per rejected request. Lives in the separate ip-attacks.db, not the
// main DB.
type Attack struct {
	ID         string `json:"id" example:"b1f6c9e2-1234-4a5b-8c9d-abcdef012345"`
	IP         string `json:"ip" example:"203.0.113.7"`
	Tier       string `json:"tier" example:"sensitive"`
	StartedAt  string `json:"started_at" example:"2026-07-26T10:00:00Z"`
	LastSeenAt string `json:"last_seen_at" example:"2026-07-26T10:04:12Z"`
	EndedAt    string `json:"ended_at,omitempty" example:"2026-07-26T10:30:00Z"`
	HitCount   int    `json:"hit_count" example:"143"`
	BanCount   int    `json:"ban_count" example:"3"`
	// OriginHint is a JSON snapshot of client-identifying request
	// headers, captured only when IP resolved to a loopback/private
	// address — populated on the detail fetch only, never on the list.
	OriginHint string `json:"origin_hint,omitempty" example:"{\"x_forwarded_for\":\"203.0.113.7\",\"host\":\"coco-iam.example.com\"}"`
}

// AttackTarget is the aggregated hit count for one endpoint (method +
// path) within a single attack episode.
type AttackTarget struct {
	Path     string `json:"path" example:"/admin/oauth/authenticate"`
	Method   string `json:"method" example:"POST"`
	HitCount int    `json:"hit_count" example:"89"`
	// BodySample is the first-observed request body for this target,
	// redacted and size-capped — nil if none was captured (a GET/HEAD
	// hit, or a content type this app doesn't capture).
	BodySample *string `json:"body_sample,omitempty" example:"{\"email\":\"admin@x.com\",\"password\":\"[REDACTED]\"}"`
}

// AttackListResponse is the list endpoint's payload — Total is the
// filtered row count (ignoring Limit/Offset), for the admin page's
// pagination.
type AttackListResponse struct {
	Attacks []Attack `json:"attacks"`
	Total   int      `json:"total" example:"37"`
	Limit   int      `json:"limit" example:"50"`
	Offset  int      `json:"offset" example:"0"`
}

// AttackDetailResponse is a single episode plus its per-endpoint
// breakdown.
type AttackDetailResponse struct {
	Attack
	Targets []AttackTarget `json:"targets"`
}

// Swag-friendly success envelopes.

type AttackListSuccess struct {
	Success bool               `json:"success" example:"true"`
	Message AttackListResponse `json:"message"`
}

type AttackDetailSuccess struct {
	Success bool                 `json:"success" example:"true"`
	Message AttackDetailResponse `json:"message"`
}
