package dbarchive

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	dbmanager "github.com/a-digi/coco-orm/orm"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"

	"github.com/a-digi/coco-iam/src/security/dbhandle"
)

// timeLayout matches ip_attacks_archives' own storage format — test-
// local since production timeLayout constants now live with whichever
// domain-specific RegistryRecorder needs them (see registry.go), not
// with the now-domain-agnostic Archiver itself.
const timeLayout = "2006-01-02 15:04:05"

// testRecorder implements RegistryRecorder against a real on-disk
// ip_attacks_archives table, mirroring the production ip-attacks
// recorder (src/admin/security/archives/repository/persistent)
// closely enough to exercise Archiver faithfully, without this
// package importing that (much larger) domain just for a test double.
type testRecorder struct {
	mainDB *sql.DB
}

func (r *testRecorder) EarliestStartedAt(db *sql.DB) string {
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

func (r *testRecorder) RecordArchive(rec ArchiveRecord) error {
	_, err := r.mainDB.Exec(
		`INSERT INTO ip_attacks_archives (id, file_path, started_at, archived_at, row_count, size_bytes)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		uuid.New().String(), rec.FilePath, rec.StartedAt, rec.ArchivedAt.Format(timeLayout), rec.RowCount, rec.SizeBytes,
	)
	return err
}

// migration001 and migration002 mirror
// api/config/db/ip_attacks_migrations/001_initial.sql and
// 002_db_meta.sql exactly — rotation exercises the real
// DatabaseManager.SyncMigrations path against real files on disk (not
// :memory:, since rotation renames the file), so the fixture needs to
// look exactly like what SyncMigrations runs in production.
const migration001 = `
CREATE TABLE IF NOT EXISTS ip_attacks
(
    id           TEXT NOT NULL CONSTRAINT ip_attacks_pk PRIMARY KEY,
    ip           TEXT NOT NULL,
    tier         TEXT NOT NULL,
    started_at   DATETIME NOT NULL,
    last_seen_at DATETIME NOT NULL,
    ended_at     DATETIME,
    hit_count    INTEGER NOT NULL DEFAULT 0,
    ban_count    INTEGER NOT NULL DEFAULT 1
);
CREATE TABLE IF NOT EXISTS ip_attack_targets
(
    id        TEXT NOT NULL CONSTRAINT ip_attack_targets_pk PRIMARY KEY,
    attack_id TEXT NOT NULL CONSTRAINT ip_attack_targets_attack_fk REFERENCES ip_attacks (id),
    path      TEXT NOT NULL,
    method    TEXT NOT NULL,
    hit_count INTEGER NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX IF NOT EXISTS ip_attack_targets_unique_idx ON ip_attack_targets (attack_id, path, method);
`

const migration002 = `
CREATE TABLE IF NOT EXISTS db_meta
(
    key   TEXT NOT NULL CONSTRAINT db_meta_pk PRIMARY KEY,
    value TEXT NOT NULL
);
INSERT INTO db_meta (key, value)
VALUES ('entry_count', '0');
`

// testFixture wires an Archiver against real on-disk SQLite files (in
// t.TempDir(), auto-cleaned) — rotation renames the live file, which
// :memory: can't exercise.
type testFixture struct {
	archiver   *Archiver
	handle     *dbhandle.Handle
	manager    *dbmanager.DatabaseManager
	mainDB     *sql.DB
	dbDir      string
	dbName     string
	archiveDir string
}

func newTestFixture(t *testing.T, threshold int64) *testFixture {
	t.Helper()
	root := t.TempDir()

	migrationsDir := filepath.Join(root, "migrations")
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		t.Fatalf("mkdir migrations dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(migrationsDir, "001_initial.sql"), []byte(migration001), 0644); err != nil {
		t.Fatalf("write migration 001: %v", err)
	}
	if err := os.WriteFile(filepath.Join(migrationsDir, "002_db_meta.sql"), []byte(migration002), 0644); err != nil {
		t.Fatalf("write migration 002: %v", err)
	}

	dbDir := filepath.Join(root, "attacksdb")
	dbName := "ip-attacks.db"
	manager, err := dbmanager.NewDatabaseManager(dbName, dbDir, []string{migrationsDir})
	if err != nil {
		t.Fatalf("NewDatabaseManager: %v", err)
	}
	if err := manager.SyncMigrations(); err != nil {
		t.Fatalf("SyncMigrations: %v", err)
	}

	handle, err := dbhandle.New(manager.Connector.DB)
	if err != nil {
		t.Fatalf("dbhandle.New: %v", err)
	}

	mainDB, err := sql.Open("sqlite3", filepath.Join(root, "main.db"))
	if err != nil {
		t.Fatalf("open main db: %v", err)
	}
	t.Cleanup(func() { _ = mainDB.Close() })
	if _, err := mainDB.Exec(`
		CREATE TABLE ip_attacks_archives (
		    id          TEXT NOT NULL PRIMARY KEY,
		    file_path   TEXT NOT NULL,
		    started_at  DATETIME NOT NULL,
		    archived_at DATETIME NOT NULL,
		    row_count   INTEGER NOT NULL,
		    size_bytes  INTEGER NOT NULL
		);
	`); err != nil {
		t.Fatalf("create ip_attacks_archives: %v", err)
	}

	archiveDir := filepath.Join(root, "archives")
	archiver := New(handle, manager, &testRecorder{mainDB: mainDB}, dbName, dbDir, migrationsDir, archiveDir, threshold, nil)

	return &testFixture{
		archiver:   archiver,
		handle:     handle,
		manager:    manager,
		mainDB:     mainDB,
		dbDir:      dbDir,
		dbName:     dbName,
		archiveDir: archiveDir,
	}
}

func (f *testFixture) livePath() string {
	return filepath.Join(f.dbDir, f.dbName)
}

func (f *testFixture) seedAttack(t *testing.T, id string, startedAt time.Time) {
	t.Helper()
	ts := startedAt.UTC().Format(timeLayout)
	if _, err := f.handle.DB().Exec(
		`INSERT INTO ip_attacks (id, ip, tier, started_at, last_seen_at, hit_count, ban_count) VALUES (?, ?, ?, ?, ?, 0, 1)`,
		id, "203.0.113.7", "global", ts, ts,
	); err != nil {
		t.Fatalf("seed attack: %v", err)
	}
	if _, err := f.handle.IncrementEntryCount(f.handle.DB(), 1); err != nil {
		t.Fatalf("increment entry count: %v", err)
	}
}

func TestCheckAndRotate_BelowThreshold_NoOp(t *testing.T) {
	f := newTestFixture(t, 10)
	f.seedAttack(t, "attack-1", time.Now())

	if err := f.archiver.CheckAndRotate(); err != nil {
		t.Fatalf("CheckAndRotate() error = %v", err)
	}

	if _, err := os.Stat(f.livePath()); err != nil {
		t.Fatalf("live db should still exist untouched: %v", err)
	}
	entries, _ := os.ReadDir(f.archiveDir)
	if len(entries) != 0 {
		t.Fatalf("archive dir should be empty below threshold, got %d entries", len(entries))
	}
	var n int
	if err := f.mainDB.QueryRow(`SELECT COUNT(*) FROM ip_attacks_archives`).Scan(&n); err != nil {
		t.Fatalf("count archives: %v", err)
	}
	if n != 0 {
		t.Fatalf("ip_attacks_archives rows = %d, want 0", n)
	}
}

func TestCheckAndRotate_AtThreshold_RotatesAndRegisters(t *testing.T) {
	f := newTestFixture(t, 3)
	oldest := time.Now().Add(-time.Hour)
	f.seedAttack(t, "attack-1", oldest)
	f.seedAttack(t, "attack-2", time.Now().Add(-30*time.Minute))
	f.seedAttack(t, "attack-3", time.Now())

	if got := f.handle.EntryCount(); got != 3 {
		t.Fatalf("EntryCount() before rotation = %d, want 3", got)
	}

	if err := f.archiver.CheckAndRotate(); err != nil {
		t.Fatalf("CheckAndRotate() error = %v", err)
	}

	// The live path now holds a fresh, empty generation.
	if got := f.handle.EntryCount(); got != 0 {
		t.Fatalf("EntryCount() after rotation = %d, want 0 (fresh generation)", got)
	}
	var attackCount int
	if err := f.handle.DB().QueryRow(`SELECT COUNT(*) FROM ip_attacks`).Scan(&attackCount); err != nil {
		t.Fatalf("count ip_attacks on fresh generation: %v", err)
	}
	if attackCount != 0 {
		t.Fatalf("fresh generation ip_attacks count = %d, want 0", attackCount)
	}

	// The old generation is registered and present on disk elsewhere.
	// started_at is read back through COALESCE to get the raw stored
	// text — a plain scan of a DATETIME-declared column round-trips
	// through the driver's own time.Time conversion and always comes
	// back as RFC3339 regardless of what was stored, the same
	// auto-conversion behavior noted on attack_query.go's
	// normalizeTimestamp.
	var filePath, startedAt string
	var rowCount, sizeBytes int64
	err := f.mainDB.QueryRow(
		`SELECT file_path, COALESCE(started_at, ''), row_count, size_bytes FROM ip_attacks_archives`,
	).Scan(&filePath, &startedAt, &rowCount, &sizeBytes)
	if err != nil {
		t.Fatalf("query ip_attacks_archives: %v", err)
	}
	if rowCount != 3 {
		t.Fatalf("registered row_count = %d, want 3", rowCount)
	}
	if sizeBytes <= 0 {
		t.Fatalf("registered size_bytes = %d, want > 0", sizeBytes)
	}
	wantStarted := oldest.UTC().Format(timeLayout)
	if startedAt != wantStarted {
		t.Fatalf("registered started_at = %q, want %q (the earliest seeded attack)", startedAt, wantStarted)
	}
	if _, err := os.Stat(filePath); err != nil {
		t.Fatalf("archived file %q should exist on disk: %v", filePath, err)
	}

	// The archived file still has the 3 original rows, untouched.
	archivedDB, err := sql.Open("sqlite3", filePath)
	if err != nil {
		t.Fatalf("open archived file: %v", err)
	}
	defer archivedDB.Close()
	var archivedCount int
	if err := archivedDB.QueryRow(`SELECT COUNT(*) FROM ip_attacks`).Scan(&archivedCount); err != nil {
		t.Fatalf("count ip_attacks in archived file: %v", err)
	}
	if archivedCount != 3 {
		t.Fatalf("archived file ip_attacks count = %d, want 3", archivedCount)
	}

	// The fresh generation is fully usable, not just empty.
	if _, err := f.handle.DB().Exec(
		`INSERT INTO ip_attacks (id, ip, tier, started_at, last_seen_at, hit_count, ban_count) VALUES (?, ?, ?, ?, ?, 0, 1)`,
		"post-rotation", "9.9.9.9", "global", time.Now().UTC().Format(timeLayout), time.Now().UTC().Format(timeLayout),
	); err != nil {
		t.Fatalf("insert into fresh generation: %v", err)
	}
}

func TestCheckAndRotate_ArchiveDirUnwritable_LiveDBUntouched(t *testing.T) {
	f := newTestFixture(t, 1)
	f.seedAttack(t, "attack-1", time.Now())

	// Replace the archive directory's parent slot with a plain file, so
	// os.MkdirAll(archiveDir, ...) fails (can't create a directory
	// where a file already occupies that path).
	if err := os.WriteFile(f.archiveDir, []byte("not a directory"), 0644); err != nil {
		t.Fatalf("seed conflicting file: %v", err)
	}

	if err := f.archiver.CheckAndRotate(); err == nil {
		t.Fatal("CheckAndRotate() error = nil, want an error (archive dir cannot be created)")
	}

	// The live connection must still be open and fully usable — the
	// failure happened before the connection was ever closed.
	var n int
	if err := f.handle.DB().QueryRow(`SELECT COUNT(*) FROM ip_attacks`).Scan(&n); err != nil {
		t.Fatalf("live db should still be queryable after a failed rotation: %v", err)
	}
	if n != 1 {
		t.Fatalf("live db row count = %d, want 1 (untouched)", n)
	}
	if got := f.handle.EntryCount(); got != 1 {
		t.Fatalf("EntryCount() after failed rotation = %d, want still 1", got)
	}
}

func TestCheckAndRotate_RenameFails_ReopensOriginal(t *testing.T) {
	f := newTestFixture(t, 1)
	f.seedAttack(t, "attack-1", time.Now())

	// Pre-create the archive dir read-only: rotate()'s own MkdirAll is
	// then a no-op (already exists), but the subsequent os.Rename into
	// it fails for lack of write permission on the directory itself.
	if err := os.MkdirAll(f.archiveDir, 0555); err != nil {
		t.Fatalf("mkdir read-only archive dir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(f.archiveDir, 0755) }) // let t.TempDir() clean up successfully

	if err := f.archiver.CheckAndRotate(); err == nil {
		t.Fatal("CheckAndRotate() error = nil, want an error (rename into a read-only directory)")
	}

	// The connection was closed to attempt the rename, then the
	// original file (never actually moved) must be reopened so the
	// process isn't left without a working ip-attacks.db.
	var n int
	if err := f.handle.DB().QueryRow(`SELECT COUNT(*) FROM ip_attacks`).Scan(&n); err != nil {
		t.Fatalf("live db should be reopened and queryable after a failed rename: %v", err)
	}
	if n != 1 {
		t.Fatalf("live db row count after failed rotation = %d, want 1 (untouched)", n)
	}
}
