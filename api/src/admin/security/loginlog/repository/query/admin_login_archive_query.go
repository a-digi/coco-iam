// Package query — this file is the read-only half of the
// admin_login.db archive-history registry. Two distinct things live
// here: the registry itself (admin_login_archives, in the main DB)
// and a helper for browsing into one specific archived admin_login.db
// file — a separate, static SQLite file per archive, opened fresh and
// read-only per request rather than held open. Mirrors
// api/src/admin/security/archives/repository/query exactly, adapted
// to this domain's table names. See plan/login-audit-log/plan.md
// Step 4.
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

// ErrArchiveNotFound signals no admin_login_archives row exists for
// the given id.
var ErrArchiveNotFound = errors.New("admin-login-archive: not found")

// AdminLoginArchiveQueryRepo reads the archive registry — always the
// main DB, which (unlike admin_login.db) is never itself rotated.
// Reuses archives_entity.ArchiveSummary: an admin_login.db archive
// registry row has the identical shape (id, started_at, archived_at,
// row_count, size_bytes) as an ip-attacks.db one, so there's no
// reason to duplicate that type just because the data lives in a
// different registry table.
type AdminLoginArchiveQueryRepo struct {
	db *sql.DB
}

func NewAdminLoginArchiveQueryRepo(db *sql.DB) *AdminLoginArchiveQueryRepo {
	return &AdminLoginArchiveQueryRepo{db: db}
}

// ListArchives returns every registered admin_login.db archive, newest
// first.
func (r *AdminLoginArchiveQueryRepo) ListArchives(limit, offset int) ([]archives_entity.ArchiveSummary, error) {
	rows, err := r.db.Query(
		`SELECT id, started_at, archived_at, row_count, size_bytes
		 FROM admin_login_archives ORDER BY archived_at DESC LIMIT ? OFFSET ?`,
		limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("admin-login-archive: list: %w", err)
	}
	defer rows.Close()

	var out []archives_entity.ArchiveSummary
	for rows.Next() {
		var a archives_entity.ArchiveSummary
		if err := rows.Scan(&a.ID, &a.StartedAt, &a.ArchivedAt, &a.RowCount, &a.SizeBytes); err != nil {
			return nil, fmt.Errorf("admin-login-archive: scan: %w", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("admin-login-archive: rows: %w", err)
	}
	return out, nil
}

// CountArchives returns the total number of registered archives, for
// the list endpoint's pagination total.
func (r *AdminLoginArchiveQueryRepo) CountArchives() (int, error) {
	var n int
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM admin_login_archives`).Scan(&n); err != nil {
		return 0, fmt.Errorf("admin-login-archive: count: %w", err)
	}
	return n, nil
}

// FindArchive returns the summary for id, plus its on-disk file path.
// The path is never serialized to a client — it's resolved here,
// server-side only, so the "browse into this archive" endpoint knows
// which file to open.
func (r *AdminLoginArchiveQueryRepo) FindArchive(id string) (archives_entity.ArchiveSummary, string, error) {
	var a archives_entity.ArchiveSummary
	var filePath string
	err := r.db.QueryRow(
		`SELECT id, file_path, started_at, archived_at, row_count, size_bytes
		 FROM admin_login_archives WHERE id = ?`,
		id,
	).Scan(&a.ID, &filePath, &a.StartedAt, &a.ArchivedAt, &a.RowCount, &a.SizeBytes)
	if errors.Is(err, sql.ErrNoRows) {
		return archives_entity.ArchiveSummary{}, "", ErrArchiveNotFound
	}
	if err != nil {
		return archives_entity.ArchiveSummary{}, "", fmt.Errorf("admin-login-archive: find: %w", err)
	}
	return a, filePath, nil
}

// OpenArchivedAttempts opens filePath read-only and returns an
// AdminLoginQueryRepo against it — the exact same query surface
// (ListAttempts/CountAttempts) the live
// /admin/security/login-log/admin endpoint uses, since an archived
// generation has the identical admin_login_attempts schema. The
// caller must Close() the returned *sql.DB once done with the
// request — this connection is opened fresh per request, never
// pooled, since archive browsing is not a hot path.
//
// See archives/repository/query.OpenArchivedAttacks's own doc comment
// for why _query_only=true plus an upfront os.Stat are both needed
// with this vendored sqlite3 driver.
func OpenArchivedAttempts(filePath string) (*AdminLoginQueryRepo, *sql.DB, error) {
	if _, err := os.Stat(filePath); err != nil {
		return nil, nil, fmt.Errorf("admin-login-archive: archived file: %w", err)
	}

	db, err := sql.Open("sqlite3", filePath+"?_query_only=true")
	if err != nil {
		return nil, nil, fmt.Errorf("admin-login-archive: open archived file: %w", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("admin-login-archive: open archived file: %w", err)
	}

	handle, err := dbhandle.New(db)
	if err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("admin-login-archive: wrap archived file: %w", err)
	}

	return NewAdminLoginQueryRepo(handle), db, nil
}
