package persistent

import (
	"database/sql"
	"testing"
	"time"

	scans_query "github.com/a-digi/coco-iam/src/admin/security/scans/repository/query"
	_ "github.com/mattn/go-sqlite3"

	"github.com/a-digi/coco-iam/src/security/dbhandle"
)

// freshDB opens an in-memory SQLite DB with the scan_episodes/db_meta
// schema — mirrors
// api/config/db/ip_attacks_migrations/003_scan_episodes.sql and
// 002_db_meta.sql.
func freshDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`
		CREATE TABLE scan_episodes (
		    id             TEXT NOT NULL CONSTRAINT scan_episodes_pk PRIMARY KEY,
		    ip             TEXT NOT NULL,
		    started_at     DATETIME NOT NULL,
		    last_seen_at   DATETIME NOT NULL,
		    ended_at       DATETIME,
		    distinct_ports INTEGER NOT NULL DEFAULT 0,
		    hit_count      INTEGER NOT NULL DEFAULT 0,
		    sample_ports   TEXT NOT NULL DEFAULT ''
		);
		CREATE TABLE db_meta (key TEXT NOT NULL PRIMARY KEY, value TEXT NOT NULL);
		INSERT INTO db_meta (key, value) VALUES ('entry_count', '0');
	`)
	if err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return db
}

func mustHandle(t *testing.T, db *sql.DB) *dbhandle.Handle {
	t.Helper()
	h, err := dbhandle.New(db)
	if err != nil {
		t.Fatalf("dbhandle.New() error = %v", err)
	}
	return h
}

func TestCreateScan_ThenFindable(t *testing.T) {
	db := freshDB(t)
	handle := mustHandle(t, db)
	persist := NewScanPersistentRepo(handle)
	query := scans_query.NewScanQueryRepo(handle)

	now := time.Now()
	if err := persist.CreateScan("scan-1", "203.0.113.7", now); err != nil {
		t.Fatalf("CreateScan() error = %v", err)
	}
	if got := handle.EntryCount(); got != 1 {
		t.Fatalf("EntryCount() after CreateScan = %d, want 1", got)
	}

	s, err := query.FindScan("scan-1")
	if err != nil {
		t.Fatalf("FindScan() error = %v", err)
	}
	if s.IP != "203.0.113.7" {
		t.Fatalf("scan = %+v, unexpected", s)
	}
	if s.DistinctPorts != 0 || s.HitCount != 0 {
		t.Fatalf("initial counts = distinct=%d hit=%d, want 0/0", s.DistinctPorts, s.HitCount)
	}
	if s.EndedAt != "" {
		t.Fatalf("EndedAt = %q, want empty for a fresh episode", s.EndedAt)
	}
}

func TestUpdateScan_FlushesLatestTotals(t *testing.T) {
	db := freshDB(t)
	handle := mustHandle(t, db)
	persist := NewScanPersistentRepo(handle)
	query := scans_query.NewScanQueryRepo(handle)

	now := time.Now()
	if err := persist.CreateScan("scan-1", "203.0.113.7", now); err != nil {
		t.Fatalf("CreateScan() error = %v", err)
	}
	if err := persist.UpdateScan("scan-1", 14, 37, "22,80,443", now.Add(time.Minute)); err != nil {
		t.Fatalf("UpdateScan() error = %v", err)
	}
	if got := handle.EntryCount(); got != 1 {
		t.Fatalf("EntryCount() after UpdateScan = %d, want still 1 (update never creates a row)", got)
	}

	s, err := query.FindScan("scan-1")
	if err != nil {
		t.Fatalf("FindScan() error = %v", err)
	}
	if s.DistinctPorts != 14 || s.HitCount != 37 || s.SamplePorts != "22,80,443" {
		t.Fatalf("scan after update = %+v, unexpected", s)
	}
}

func TestCloseScan_SetsEndedAt(t *testing.T) {
	db := freshDB(t)
	handle := mustHandle(t, db)
	persist := NewScanPersistentRepo(handle)
	query := scans_query.NewScanQueryRepo(handle)

	now := time.Now()
	if err := persist.CreateScan("scan-1", "203.0.113.7", now); err != nil {
		t.Fatalf("CreateScan() error = %v", err)
	}
	if err := persist.CloseScan("scan-1", now.Add(5*time.Minute)); err != nil {
		t.Fatalf("CloseScan() error = %v", err)
	}

	s, err := query.FindScan("scan-1")
	if err != nil {
		t.Fatalf("FindScan() error = %v", err)
	}
	if s.EndedAt == "" {
		t.Fatal("EndedAt should be set after CloseScan")
	}
}

func TestCloseAllOpen_ClosesOnlyOpenRows(t *testing.T) {
	db := freshDB(t)
	handle := mustHandle(t, db)
	persist := NewScanPersistentRepo(handle)
	query := scans_query.NewScanQueryRepo(handle)

	now := time.Now()
	if err := persist.CreateScan("open-1", "1.1.1.1", now); err != nil {
		t.Fatalf("CreateScan() error = %v", err)
	}
	if err := persist.CreateScan("open-2", "2.2.2.2", now); err != nil {
		t.Fatalf("CreateScan() error = %v", err)
	}
	if err := persist.CreateScan("already-closed", "3.3.3.3", now); err != nil {
		t.Fatalf("CreateScan() error = %v", err)
	}
	if err := persist.CloseScan("already-closed", now); err != nil {
		t.Fatalf("CloseScan() error = %v", err)
	}

	n, err := persist.CloseAllOpen()
	if err != nil {
		t.Fatalf("CloseAllOpen() error = %v", err)
	}
	if n != 2 {
		t.Fatalf("CloseAllOpen() closed %d rows, want 2", n)
	}

	for _, id := range []string{"open-1", "open-2", "already-closed"} {
		s, err := query.FindScan(id)
		if err != nil {
			t.Fatalf("FindScan(%s) error = %v", id, err)
		}
		if s.EndedAt == "" {
			t.Fatalf("scan %s should be closed", id)
		}
	}
}

func TestListScans_NewestFirstAndFiltered(t *testing.T) {
	db := freshDB(t)
	handle := mustHandle(t, db)
	persist := NewScanPersistentRepo(handle)
	query := scans_query.NewScanQueryRepo(handle)

	base := time.Now()
	if err := persist.CreateScan("scan-a", "1.1.1.1", base.Add(-2*time.Hour)); err != nil {
		t.Fatalf("CreateScan() error = %v", err)
	}
	if err := persist.CreateScan("scan-b", "2.2.2.2", base.Add(-1*time.Hour)); err != nil {
		t.Fatalf("CreateScan() error = %v", err)
	}
	if err := persist.CreateScan("scan-c", "1.1.1.1", base); err != nil {
		t.Fatalf("CreateScan() error = %v", err)
	}

	all, err := query.ListScans(scans_query.ListFilter{Limit: 50})
	if err != nil {
		t.Fatalf("ListScans() error = %v", err)
	}
	if len(all) != 3 || all[0].ID != "scan-c" || all[2].ID != "scan-a" {
		t.Fatalf("ListScans() = %+v, want newest-first [scan-c,scan-b,scan-a]", all)
	}

	filtered, err := query.ListScans(scans_query.ListFilter{IP: "1.1.1.1", Limit: 50})
	if err != nil {
		t.Fatalf("ListScans(ip filter) error = %v", err)
	}
	if len(filtered) != 2 {
		t.Fatalf("ListScans(ip=1.1.1.1) = %+v, want 2 rows", filtered)
	}

	total, err := query.CountScans(scans_query.ListFilter{IP: "1.1.1.1"})
	if err != nil {
		t.Fatalf("CountScans() error = %v", err)
	}
	if total != 2 {
		t.Fatalf("CountScans(ip=1.1.1.1) = %d, want 2", total)
	}
}

func TestFindScan_NotFound(t *testing.T) {
	db := freshDB(t)
	handle := mustHandle(t, db)
	query := scans_query.NewScanQueryRepo(handle)
	if _, err := query.FindScan("nope"); err != scans_query.ErrNotFound {
		t.Fatalf("FindScan() error = %v, want ErrNotFound", err)
	}
}
