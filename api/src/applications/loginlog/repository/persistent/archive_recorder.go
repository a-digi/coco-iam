// Package persistent is the write-only half of the per-application
// login-log — both application_login_attempts rows themselves (Step 7)
// and, here, the dbarchive.RegistryRecorder implementation that knows
// a <slug>_login.db's own schema (application_login_attempts.created_at)
// and where its registry table lives: application_login_archives,
// shared across every application in the owning organization's own
// users.db (filtered by application_id), not a table per application.
// See plan/login-audit-log/plan.md Step 6.
package persistent

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/a-digi/coco-iam/src/security/dbarchive"
)

// timeLayout matches application_login_archives' own storage format -
// each package in this codebase keeps its own copy of this constant
// rather than sharing one, the established convention.
const timeLayout = "2006-01-02 15:04:05"

// ArchiveRecorder implements dbarchive.RegistryRecorder against
// application_login_archives in one organization's own users.db,
// scoped to one application via applicationID.
type ArchiveRecorder struct {
	orgDB         *sql.DB
	applicationID string
}

func NewArchiveRecorder(orgDB *sql.DB, applicationID string) *ArchiveRecorder {
	return &ArchiveRecorder{orgDB: orgDB, applicationID: applicationID}
}

// EarliestStartedAt returns the earliest
// application_login_attempts.created_at in db, formatted for storage
// in application_login_archives - or "now" if this generation never
// recorded a login attempt. Parsed defensively against either the
// driver's own RFC3339 round-trip or the raw stored layout, the same
// way the admin-login and ip-attacks recorders do.
func (r *ArchiveRecorder) EarliestStartedAt(db *sql.DB) string {
	var raw sql.NullString
	if err := db.QueryRow(`SELECT MIN(created_at) FROM application_login_attempts`).Scan(&raw); err != nil || !raw.Valid {
		return time.Now().UTC().Format(timeLayout)
	}
	for _, layout := range []string{timeLayout, time.RFC3339, time.RFC3339Nano} {
		if t, err := time.Parse(layout, raw.String); err == nil {
			return t.UTC().Format(timeLayout)
		}
	}
	return time.Now().UTC().Format(timeLayout)
}

// RecordArchive inserts one row into application_login_archives,
// tagged with the application this recorder was built for.
func (r *ArchiveRecorder) RecordArchive(rec dbarchive.ArchiveRecord) error {
	_, err := r.orgDB.Exec(
		`INSERT INTO application_login_archives (id, application_id, file_path, started_at, archived_at, row_count, size_bytes)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		uuid.New().String(), r.applicationID, rec.FilePath, rec.StartedAt, rec.ArchivedAt.Format(timeLayout), rec.RowCount, rec.SizeBytes,
	)
	if err != nil {
		return fmt.Errorf("insert application_login_archives row: %w", err)
	}
	return nil
}
