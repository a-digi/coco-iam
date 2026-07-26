// Package entity holds the request/response types for the read-only
// admin ip-attacks.db archive-history API. See
// plan/ip-attacks-db-archiving/plan.md.
package entity

// ArchiveSummary is one rotated-out ip-attacks.db generation, tracked
// in the main DB's ip_attacks_archives table so it stays queryable
// long after the file itself was moved out of the live path. The
// underlying file path is a server-side implementation detail, never
// serialized here — clients only ever address an archive by id.
type ArchiveSummary struct {
	ID         string `json:"id" example:"b1f6c9e2-1234-4a5b-8c9d-abcdef012345"`
	StartedAt  string `json:"started_at" example:"2026-06-01T00:00:00Z"`
	ArchivedAt string `json:"archived_at" example:"2026-07-26T10:00:00Z"`
	RowCount   int64  `json:"row_count" example:"100000000"`
	SizeBytes  int64  `json:"size_bytes" example:"5368709120"`
}

// ArchiveListResponse is the list endpoint's payload — Total is the
// total archive count (ignoring Limit/Offset), for the admin page's
// pagination.
type ArchiveListResponse struct {
	Archives []ArchiveSummary `json:"archives"`
	Total    int              `json:"total" example:"3"`
	Limit    int              `json:"limit" example:"50"`
	Offset   int              `json:"offset" example:"0"`
}

// Swag-friendly success envelopes.

type ArchiveListSuccess struct {
	Success bool                `json:"success" example:"true"`
	Message ArchiveListResponse `json:"message"`
}

type ArchiveDetailSuccess struct {
	Success bool           `json:"success" example:"true"`
	Message ArchiveSummary `json:"message"`
}
