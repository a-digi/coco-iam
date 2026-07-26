package query

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	attacks_query "github.com/a-digi/coco-iam/src/admin/security/attacks/repository/query"
	_ "github.com/mattn/go-sqlite3"
)

// freshMainDB opens an in-memory SQLite DB with the ip_attacks_archives
// schema — mirrors api/config/db/migrations/26_07_2026_11_00_00.sql.
func freshMainDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(`
		CREATE TABLE ip_attacks_archives (
		    id          TEXT NOT NULL PRIMARY KEY,
		    file_path   TEXT NOT NULL,
		    started_at  DATETIME NOT NULL,
		    archived_at DATETIME NOT NULL,
		    row_count   INTEGER NOT NULL,
		    size_bytes  INTEGER NOT NULL
		);
	`); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return db
}

func seedArchive(t *testing.T, db *sql.DB, id, filePath string, archivedAt time.Time, rowCount, sizeBytes int64) {
	t.Helper()
	ts := archivedAt.UTC().Format("2006-01-02 15:04:05")
	if _, err := db.Exec(
		`INSERT INTO ip_attacks_archives (id, file_path, started_at, archived_at, row_count, size_bytes)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		id, filePath, ts, ts, rowCount, sizeBytes,
	); err != nil {
		t.Fatalf("seed archive: %v", err)
	}
}

func TestListArchives_NewestFirstAndPaginated(t *testing.T) {
	db := freshMainDB(t)
	repo := NewArchiveQueryRepo(db)

	base := time.Now()
	seedArchive(t, db, "a1", "/archives/a1.db", base.Add(-2*time.Hour), 100, 1024)
	seedArchive(t, db, "a2", "/archives/a2.db", base.Add(-1*time.Hour), 200, 2048)
	seedArchive(t, db, "a3", "/archives/a3.db", base, 300, 4096)

	all, err := repo.ListArchives(50, 0)
	if err != nil {
		t.Fatalf("ListArchives() error = %v", err)
	}
	if len(all) != 3 || all[0].ID != "a3" || all[2].ID != "a1" {
		t.Fatalf("ListArchives() = %+v, want newest-first [a3,a2,a1]", all)
	}

	page, err := repo.ListArchives(1, 1)
	if err != nil {
		t.Fatalf("ListArchives(paginated) error = %v", err)
	}
	if len(page) != 1 || page[0].ID != "a2" {
		t.Fatalf("ListArchives(limit=1,offset=1) = %+v, want [a2]", page)
	}

	total, err := repo.CountArchives()
	if err != nil {
		t.Fatalf("CountArchives() error = %v", err)
	}
	if total != 3 {
		t.Fatalf("CountArchives() = %d, want 3", total)
	}
}

func TestFindArchive_ReturnsSummaryAndFilePath(t *testing.T) {
	db := freshMainDB(t)
	repo := NewArchiveQueryRepo(db)
	seedArchive(t, db, "a1", "/archives/a1.db", time.Now(), 100, 1024)

	summary, filePath, err := repo.FindArchive("a1")
	if err != nil {
		t.Fatalf("FindArchive() error = %v", err)
	}
	if summary.ID != "a1" || summary.RowCount != 100 || summary.SizeBytes != 1024 {
		t.Fatalf("FindArchive() summary = %+v, unexpected", summary)
	}
	if filePath != "/archives/a1.db" {
		t.Fatalf("FindArchive() filePath = %q, want /archives/a1.db", filePath)
	}
}

func TestFindArchive_NotFound(t *testing.T) {
	db := freshMainDB(t)
	repo := NewArchiveQueryRepo(db)
	if _, _, err := repo.FindArchive("nope"); err != ErrNotFound {
		t.Fatalf("FindArchive() error = %v, want ErrNotFound", err)
	}
}

// newArchivedFile creates a real on-disk SQLite file with the
// ip_attacks/ip_attack_targets/db_meta schema and one seeded row —
// OpenArchivedAttacks needs a real file (mode=ro is a filesystem-level
// open flag, not meaningful against :memory:).
func newArchivedFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "archived.db")
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("create archived file: %v", err)
	}
	if _, err := db.Exec(`
		CREATE TABLE ip_attacks (
		    id           TEXT NOT NULL CONSTRAINT ip_attacks_pk PRIMARY KEY,
		    ip           TEXT NOT NULL,
		    tier         TEXT NOT NULL,
		    started_at   DATETIME NOT NULL,
		    last_seen_at DATETIME NOT NULL,
		    ended_at     DATETIME,
		    hit_count    INTEGER NOT NULL DEFAULT 0,
		    ban_count    INTEGER NOT NULL DEFAULT 1
		);
		CREATE TABLE ip_attack_targets (
		    id        TEXT NOT NULL CONSTRAINT ip_attack_targets_pk PRIMARY KEY,
		    attack_id TEXT NOT NULL,
		    path      TEXT NOT NULL,
		    method    TEXT NOT NULL,
		    hit_count INTEGER NOT NULL DEFAULT 0
		);
		CREATE TABLE db_meta (key TEXT NOT NULL PRIMARY KEY, value TEXT NOT NULL);
		INSERT INTO db_meta (key, value) VALUES ('entry_count', '1');
	`); err != nil {
		t.Fatalf("create archived schema: %v", err)
	}
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	if _, err := db.Exec(
		`INSERT INTO ip_attacks (id, ip, tier, started_at, last_seen_at, hit_count, ban_count) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"archived-attack-1", "203.0.113.7", "global", now, now, 42, 1,
	); err != nil {
		t.Fatalf("seed archived attack: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close after seeding: %v", err)
	}
	return path
}

func TestOpenArchivedAttacks_ReadsSeededData(t *testing.T) {
	path := newArchivedFile(t)

	repo, archiveDB, err := OpenArchivedAttacks(path)
	if err != nil {
		t.Fatalf("OpenArchivedAttacks() error = %v", err)
	}
	defer archiveDB.Close()

	attack, err := repo.FindAttack("archived-attack-1")
	if err != nil {
		t.Fatalf("FindAttack() error = %v", err)
	}
	if attack.IP != "203.0.113.7" || attack.HitCount != 42 {
		t.Fatalf("FindAttack() = %+v, unexpected", attack)
	}

	attacks, err := repo.ListAttacks(attacks_query.ListFilter{Limit: 50})
	if err != nil {
		t.Fatalf("ListAttacks() error = %v", err)
	}
	if len(attacks) != 1 {
		t.Fatalf("ListAttacks() = %+v, want 1 row", attacks)
	}
}

func TestOpenArchivedAttacks_IsReadOnly(t *testing.T) {
	path := newArchivedFile(t)

	_, archiveDB, err := OpenArchivedAttacks(path)
	if err != nil {
		t.Fatalf("OpenArchivedAttacks() error = %v", err)
	}
	defer archiveDB.Close()

	_, err = archiveDB.Exec(`INSERT INTO ip_attacks (id, ip, tier, started_at, last_seen_at) VALUES ('x', 'y', 'z', '', '')`)
	if err == nil {
		t.Fatal("write against a mode=ro archived connection should fail, got nil error")
	}
}

func TestOpenArchivedAttacks_MissingFile(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist.db")
	if _, err := os.Stat(missing); err == nil {
		t.Fatal("test setup: file unexpectedly exists")
	}

	if _, _, err := OpenArchivedAttacks(missing); err == nil {
		t.Fatal("OpenArchivedAttacks() on a missing file: error = nil, want an error")
	}
}
