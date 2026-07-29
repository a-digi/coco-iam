// Package query — this file is the read-only half of the
// per-application login-log archive-history registry.
// application_login_archives lives in the owning organization's own
// users.db, shared across every application in that org (filtered by
// application_id) rather than one table per application. Mirrors
// api/src/admin/security/archives/repository/query and
// api/src/admin/security/loginlog/repository/query's own archive
// query, adapted to this domain's table/filter shape. See
// plan/login-audit-log/plan.md Step 8.
package query

import (
	"database/sql"
	"errors"
	"fmt"
	"os"

	_ "github.com/mattn/go-sqlite3"

	archives_entity "github.com/a-digi/coco-iam/src/admin/security/archives/entity"
	"github.com/a-digi/coco-iam/src/security/dbhandle"
)

// ErrArchiveNotFound signals no application_login_archives row exists
// for the given id (scoped to one application).
var ErrArchiveNotFound = errors.New("application-login-archive: not found")

// ApplicationLoginArchiveQueryRepo reads the archive registry — always
// the owning organization's users.db, which (unlike a
// <slug>_login.db) is never itself rotated. Every query is additionally
// filtered by applicationID, so one application's admin can never see
// another application's archive rows even though they share the same
// underlying table. Reuses archives_entity.ArchiveSummary: identical
// shape (id, started_at, archived_at, row_count, size_bytes) to every
// other archive registry in this codebase.
type ApplicationLoginArchiveQueryRepo struct {
	orgDB         *sql.DB
	applicationID string
}

func NewApplicationLoginArchiveQueryRepo(orgDB *sql.DB, applicationID string) *ApplicationLoginArchiveQueryRepo {
	return &ApplicationLoginArchiveQueryRepo{orgDB: orgDB, applicationID: applicationID}
}

// ListArchives returns every registered archive for this application,
// newest first.
func (r *ApplicationLoginArchiveQueryRepo) ListArchives(limit, offset int) ([]archives_entity.ArchiveSummary, error) {
	rows, err := r.orgDB.Query(
		`SELECT id, started_at, archived_at, row_count, size_bytes
		 FROM application_login_archives WHERE application_id = ?
		 ORDER BY archived_at DESC LIMIT ? OFFSET ?`,
		r.applicationID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("application-login-archive: list: %w", err)
	}
	defer rows.Close()

	var out []archives_entity.ArchiveSummary
	for rows.Next() {
		var a archives_entity.ArchiveSummary
		if err := rows.Scan(&a.ID, &a.StartedAt, &a.ArchivedAt, &a.RowCount, &a.SizeBytes); err != nil {
			return nil, fmt.Errorf("application-login-archive: scan: %w", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("application-login-archive: rows: %w", err)
	}
	return out, nil
}

// CountArchives returns the total number of registered archives for
// this application, for the list endpoint's pagination total.
func (r *ApplicationLoginArchiveQueryRepo) CountArchives() (int, error) {
	var n int
	if err := r.orgDB.QueryRow(
		`SELECT COUNT(*) FROM application_login_archives WHERE application_id = ?`, r.applicationID,
	).Scan(&n); err != nil {
		return 0, fmt.Errorf("application-login-archive: count: %w", err)
	}
	return n, nil
}

// FindArchive returns the summary for id, plus its on-disk file path —
// scoped to this application, so an archive id belonging to a
// different application (even in the same org) is treated as not
// found. The path is never serialized to a client.
func (r *ApplicationLoginArchiveQueryRepo) FindArchive(id string) (archives_entity.ArchiveSummary, string, error) {
	var a archives_entity.ArchiveSummary
	var filePath string
	err := r.orgDB.QueryRow(
		`SELECT id, file_path, started_at, archived_at, row_count, size_bytes
		 FROM application_login_archives WHERE id = ? AND application_id = ?`,
		id, r.applicationID,
	).Scan(&a.ID, &filePath, &a.StartedAt, &a.ArchivedAt, &a.RowCount, &a.SizeBytes)
	if errors.Is(err, sql.ErrNoRows) {
		return archives_entity.ArchiveSummary{}, "", ErrArchiveNotFound
	}
	if err != nil {
		return archives_entity.ArchiveSummary{}, "", fmt.Errorf("application-login-archive: find: %w", err)
	}
	return a, filePath, nil
}

// OpenArchivedAttempts opens filePath read-only and returns an
// ApplicationLoginQueryRepo against it — the exact same query surface
// (ListAttempts/CountAttempts) the live per-application login-log
// endpoint uses, since an archived generation has the identical
// application_login_attempts schema. The caller must Close() the
// returned *sql.DB once done with the request.
func OpenArchivedAttempts(filePath string) (*ApplicationLoginQueryRepo, *sql.DB, error) {
	if _, err := os.Stat(filePath); err != nil {
		return nil, nil, fmt.Errorf("application-login-archive: archived file: %w", err)
	}

	db, err := sql.Open("sqlite3", filePath+"?_query_only=true")
	if err != nil {
		return nil, nil, fmt.Errorf("application-login-archive: open archived file: %w", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("application-login-archive: open archived file: %w", err)
	}

	handle, err := dbhandle.New(db)
	if err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("application-login-archive: wrap archived file: %w", err)
	}

	return NewApplicationLoginQueryRepo(handle), db, nil
}
