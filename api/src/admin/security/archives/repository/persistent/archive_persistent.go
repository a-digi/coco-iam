// Package persistent is the write-only half of the ip-attacks archive
// registry — the dbarchive.RegistryRecorder implementation that knows
// ip-attacks.db's own schema (ip_attacks.started_at) and where its
// registry table lives (ip_attacks_archives, in the main DB). See
// plan/ip-attacks-db-archiving/plan.md and plan/login-audit-log/plan.md
// Step 1.
package persistent

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/a-digi/coco-iam/src/security/dbarchive"
)

// timeLayout matches ip_attacks_archives' own storage format — see
// dbarchive's and AttackPersistentRepo's own timeLayout constants;
// each package keeps its own copy rather than sharing one, the same
// convention already used throughout this codebase.
const timeLayout = "2006-01-02 15:04:05"

// ArchiveRecorder implements dbarchive.RegistryRecorder against
// ip_attacks_archives in the main DB.
type ArchiveRecorder struct {
	mainDB *sql.DB
}

func NewArchiveRecorder(mainDB *sql.DB) *ArchiveRecorder {
	return &ArchiveRecorder{mainDB: mainDB}
}

// EarliestStartedAt returns the earliest ip_attacks.started_at in db,
// formatted for storage in ip_attacks_archives — or "now" if this
// generation never recorded an attack (nothing to derive a start time
// from). The driver recognizes MIN()'s result as the column's
// DATETIME type and hands back RFC3339 (unlike, say, wrapping a column
// in COALESCE, which loses that type metadata and comes back as a
// plain string) — parsed defensively against either shape anyway, the
// same way ipguard's own parseTime does for security_ip_bans.expires_at.
func (r *ArchiveRecorder) EarliestStartedAt(db *sql.DB) string {
	var raw sql.NullString
	if err := db.QueryRow(`SELECT MIN(started_at) FROM ip_attacks`).Scan(&raw); err != nil || !raw.Valid {
		return time.Now().UTC().Format(timeLayout)
	}
	for _, layout := range []string{timeLayout, time.RFC3339, time.RFC3339Nano} {
		if t, err := time.Parse(layout, raw.String); err == nil {
			return t.UTC().Format(timeLayout)
		}
	}
	return time.Now().UTC().Format(timeLayout)
}

// RecordArchive inserts one row into ip_attacks_archives.
func (r *ArchiveRecorder) RecordArchive(rec dbarchive.ArchiveRecord) error {
	_, err := r.mainDB.Exec(
		`INSERT INTO ip_attacks_archives (id, file_path, started_at, archived_at, row_count, size_bytes)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		uuid.New().String(), rec.FilePath, rec.StartedAt, rec.ArchivedAt.Format(timeLayout), rec.RowCount, rec.SizeBytes,
	)
	if err != nil {
		return fmt.Errorf("insert ip_attacks_archives row: %w", err)
	}
	return nil
}
