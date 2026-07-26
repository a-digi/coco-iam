// Package query is the read-only half of the archive-history
// repository. Two distinct things live here: the registry itself
// (ip_attacks_archives, in the main DB, never rotated) and a helper for
// browsing into one specific archived ip-attacks.db file — a separate,
// static SQLite file per archive, opened fresh and read-only per
// request rather than held open. See
// plan/ip-attacks-db-archiving/plan.md.
package query

import (
	"database/sql"
	"errors"
	"fmt"
	"os"

	_ "github.com/mattn/go-sqlite3"

	archives_entity "github.com/a-digi/coco-iam/src/admin/security/archives/entity"
	attacks_query "github.com/a-digi/coco-iam/src/admin/security/attacks/repository/query"
	"github.com/a-digi/coco-iam/src/security/dbhandle"
)

// ErrNotFound signals no ip_attacks_archives row exists for the given id.
var ErrNotFound = errors.New("ip-attacks-archive: not found")

// ArchiveQueryRepo reads the archive registry — always the main DB,
// which (unlike ip-attacks.db) is never itself rotated.
type ArchiveQueryRepo struct {
	db *sql.DB
}

func NewArchiveQueryRepo(db *sql.DB) *ArchiveQueryRepo {
	return &ArchiveQueryRepo{db: db}
}

// ListArchives returns every registered archive, newest first.
func (r *ArchiveQueryRepo) ListArchives(limit, offset int) ([]archives_entity.ArchiveSummary, error) {
	rows, err := r.db.Query(
		`SELECT id, started_at, archived_at, row_count, size_bytes
		 FROM ip_attacks_archives ORDER BY archived_at DESC LIMIT ? OFFSET ?`,
		limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("ip-attacks-archive: list: %w", err)
	}
	defer rows.Close()

	var out []archives_entity.ArchiveSummary
	for rows.Next() {
		var a archives_entity.ArchiveSummary
		if err := rows.Scan(&a.ID, &a.StartedAt, &a.ArchivedAt, &a.RowCount, &a.SizeBytes); err != nil {
			return nil, fmt.Errorf("ip-attacks-archive: scan: %w", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ip-attacks-archive: rows: %w", err)
	}
	return out, nil
}

// CountArchives returns the total number of registered archives, for
// the list endpoint's pagination total.
func (r *ArchiveQueryRepo) CountArchives() (int, error) {
	var n int
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM ip_attacks_archives`).Scan(&n); err != nil {
		return 0, fmt.Errorf("ip-attacks-archive: count: %w", err)
	}
	return n, nil
}

// FindArchive returns the summary for id, plus its on-disk file path.
// The path is never serialized to a client — it's resolved here,
// server-side only, so the "browse into this archive" endpoints know
// which file to open.
func (r *ArchiveQueryRepo) FindArchive(id string) (archives_entity.ArchiveSummary, string, error) {
	var a archives_entity.ArchiveSummary
	var filePath string
	err := r.db.QueryRow(
		`SELECT id, file_path, started_at, archived_at, row_count, size_bytes
		 FROM ip_attacks_archives WHERE id = ?`,
		id,
	).Scan(&a.ID, &filePath, &a.StartedAt, &a.ArchivedAt, &a.RowCount, &a.SizeBytes)
	if errors.Is(err, sql.ErrNoRows) {
		return archives_entity.ArchiveSummary{}, "", ErrNotFound
	}
	if err != nil {
		return archives_entity.ArchiveSummary{}, "", fmt.Errorf("ip-attacks-archive: find: %w", err)
	}
	return a, filePath, nil
}

// OpenArchivedAttacks opens filePath read-only and returns an
// AttackQueryRepo against it — the exact same query surface
// (ListAttacks/CountAttacks/FindAttack/ListTargets) the live
// /admin/security/attacks endpoints use, since an archived generation
// has the identical ip_attacks/ip_attack_targets schema. The caller
// must Close() the returned *sql.DB once done with the request — this
// connection is opened fresh per request, never pooled, since archive
// browsing is not a hot path.
//
// This vendored mattn/go-sqlite3 always opens with
// SQLITE_OPEN_READWRITE|SQLITE_OPEN_CREATE regardless of DSN (it has
// no "mode=ro" URI support) and would otherwise silently create an
// empty file at a wrong/stale path — filePath is stat'd first so a
// bad path fails loudly instead. _query_only=true then has the
// driver itself run "PRAGMA query_only = 1" on open, rejecting any
// write against this connection at the SQL level (belt-and-braces
// alongside the fact that nothing in AttackQueryRepo ever writes).
func OpenArchivedAttacks(filePath string) (*attacks_query.AttackQueryRepo, *sql.DB, error) {
	if _, err := os.Stat(filePath); err != nil {
		return nil, nil, fmt.Errorf("ip-attacks-archive: archived file: %w", err)
	}

	db, err := sql.Open("sqlite3", filePath+"?_query_only=true")
	if err != nil {
		return nil, nil, fmt.Errorf("ip-attacks-archive: open archived file: %w", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("ip-attacks-archive: open archived file: %w", err)
	}

	handle, err := dbhandle.New(db)
	if err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("ip-attacks-archive: wrap archived file: %w", err)
	}

	return attacks_query.NewAttackQueryRepo(handle), db, nil
}
