// Package entity holds the request/response types for the read-only
// admin attack-history API. See plan/ip-abuse-protection/plan.md
// sections 10 and 13.
//
// Attack/AttackTarget themselves now live in
// github.com/a-digi/coco-sec/ipguard/entity (see
// plan/coco-sec-extraction/plan.md) — this package keeps only the
// list/detail response envelopes, referencing the library's types
// directly.
package entity

import (
	cocosecentity "github.com/a-digi/coco-sec/ipguard/entity"
)

// AttackListResponse is the list endpoint's payload — Total is the
// filtered row count (ignoring Limit/Offset), for the admin page's
// pagination.
type AttackListResponse struct {
	Attacks []cocosecentity.Attack `json:"attacks"`
	Total   int                    `json:"total" example:"37"`
	Limit   int                    `json:"limit" example:"50"`
	Offset  int                    `json:"offset" example:"0"`
}

// AttackDetailResponse is a single episode plus its per-endpoint
// breakdown.
type AttackDetailResponse struct {
	cocosecentity.Attack
	Targets []cocosecentity.AttackTarget `json:"targets"`
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
