// Package persistent is the write-only half of the admin login log —
// both the admin_login_attempts rows themselves (added in
// plan/login-audit-log/plan.md Step 3) and, here, the
// dbarchive.RegistryRecorder implementation that knows admin_login.db's
// own schema (admin_login_attempts.created_at) and where its registry
// table lives (admin_login_archives, in the main DB). Mirrors
// api/src/admin/security/archives/repository/persistent exactly,
// adapted to this domain's table names. See
// plan/login-audit-log/plan.md Step 2.
package persistent

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/a-digi/coco-iam/src/security/dbarchive"
)

// timeLayout matches admin_login_archives' own storage format — each
// package in this codebase keeps its own copy of this constant rather
// than sharing one; see dbarchive's and AttackPersistentRepo's own
// timeLayout constants for the established convention.
const timeLayout = "2006-01-02 15:04:05"

// ArchiveRecorder implements dbarchive.RegistryRecorder against
// admin_login_archives in the main DB.
type ArchiveRecorder struct {
	mainDB *sql.DB
}

func NewArchiveRecorder(mainDB *sql.DB) *ArchiveRecorder {
	return &ArchiveRecorder{mainDB: mainDB}
}

// EarliestStartedAt returns the earliest
// admin_login_attempts.created_at in db, formatted for storage in
// admin_login_archives — or "now" if this generation never recorded a
// login attempt (nothing to derive a start time from). Parsed
// defensively against either the driver's own RFC3339 round-trip or
// the raw stored layout, the same way dbarchive's ip-attacks recorder
// does for ip_attacks.started_at.
func (r *ArchiveRecorder) EarliestStartedAt(db *sql.DB) string {
	var raw sql.NullString
	if err := db.QueryRow(`SELECT MIN(created_at) FROM admin_login_attempts`).Scan(&raw); err != nil || !raw.Valid {
		return time.Now().UTC().Format(timeLayout)
	}
	for _, layout := range []string{timeLayout, time.RFC3339, time.RFC3339Nano} {
		if t, err := time.Parse(layout, raw.String); err == nil {
			return t.UTC().Format(timeLayout)
		}
	}
	return time.Now().UTC().Format(timeLayout)
}

// RecordArchive inserts one row into admin_login_archives.
func (r *ArchiveRecorder) RecordArchive(rec dbarchive.ArchiveRecord) error {
	_, err := r.mainDB.Exec(
		`INSERT INTO admin_login_archives (id, file_path, started_at, archived_at, row_count, size_bytes)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		uuid.New().String(), rec.FilePath, rec.StartedAt, rec.ArchivedAt.Format(timeLayout), rec.RowCount, rec.SizeBytes,
	)
	if err != nil {
		return fmt.Errorf("insert admin_login_archives row: %w", err)
	}
	return nil
}
