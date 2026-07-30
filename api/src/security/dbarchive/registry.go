package dbarchive

import (
	"database/sql"
	"time"
)

// ArchiveRecord describes one file that was just rotated out of the
// live path — everything a registry table needs to remember about it.
type ArchiveRecord struct {
	FilePath   string
	StartedAt  string
	ArchivedAt time.Time
	RowCount   int64
	SizeBytes  int64
}

// RegistryRecorder is the domain-specific half of a rotation: knowing
// where in the live database to derive a "this generation's window
// started at" timestamp from, and where to persist the record of a
// rotated-out file so it stays queryable later. Archiver itself holds
// no opinion on either — different domains keep their registry table
// in entirely different databases (the main DB for ip-attacks.db and
// admin_login.db; the owning organization's users.db for a
// per-application login log), so there is no single "the" registry
// database Archiver could hold a connection to itself. See
// plan/login-audit-log/plan.md Step 1.
type RegistryRecorder interface {
	// EarliestStartedAt returns the earliest meaningful timestamp in
	// db for the generation about to be rotated out, formatted for
	// storage — e.g. MIN(started_at) FROM ip_attacks. Returns "now"
	// if the generation has no rows to derive a start time from.
	// Called while db is still the live connection, immediately
	// before rotation begins.
	EarliestStartedAt(db *sql.DB) string

	// RecordArchive persists one row describing a rotated-out file.
	RecordArchive(rec ArchiveRecord) error
}
