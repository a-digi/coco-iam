package persistent

import (
	"database/sql"
	"testing"
	"time"

	security_query "github.com/a-digi/coco-iam/src/admin/security/repository/query"
	_ "github.com/mattn/go-sqlite3"
)

// freshDB opens an in-memory SQLite DB with the ip_bans/ip_allowlist
// schema — mirrors api/config/db/migrations/26_07_2026_10_00_00.sql.
func freshDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`
		CREATE TABLE ip_bans (
		    ip         TEXT NOT NULL CONSTRAINT ip_bans_pk PRIMARY KEY,
		    tier       TEXT NOT NULL,
		    reason     TEXT NOT NULL,
		    banned_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		    expires_at DATETIME NOT NULL,
		    hit_count  INTEGER NOT NULL DEFAULT 1,
		    created_by TEXT
		);
		CREATE INDEX ip_bans_expires_at_idx ON ip_bans (expires_at);
		CREATE TABLE ip_allowlist (
		    ip         TEXT NOT NULL CONSTRAINT ip_allowlist_pk PRIMARY KEY,
		    note       TEXT,
		    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		    created_by TEXT NOT NULL
		);
	`)
	if err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return db
}

func TestUpsertBan_CreatesThenIncrementsHitCountOnRepeat(t *testing.T) {
	db := freshDB(t)
	persist := NewIPBanPersistentRepo(db)
	query := security_query.NewIPBanQueryRepo(db)

	expiresAt := time.Now().Add(15 * time.Minute)
	if err := persist.UpsertBan("203.0.113.7", "global", "ddos rate limit exceeded", expiresAt, nil); err != nil {
		t.Fatalf("UpsertBan() first call error = %v", err)
	}
	b, err := query.FindBan("203.0.113.7")
	if err != nil {
		t.Fatalf("FindBan() error = %v", err)
	}
	if b.HitCount != 1 {
		t.Fatalf("HitCount after first ban = %d, want 1", b.HitCount)
	}
	if b.CreatedBy != "" {
		t.Fatalf("CreatedBy = %q, want empty for an auto-ban", b.CreatedBy)
	}

	if err := persist.UpsertBan("203.0.113.7", "sensitive", "sensitive rate limit exceeded", expiresAt.Add(time.Hour), nil); err != nil {
		t.Fatalf("UpsertBan() second call error = %v", err)
	}
	b, err = query.FindBan("203.0.113.7")
	if err != nil {
		t.Fatalf("FindBan() after repeat error = %v", err)
	}
	if b.HitCount != 2 {
		t.Fatalf("HitCount after repeat offense = %d, want 2 (incremented, not duplicated)", b.HitCount)
	}
	if b.Tier != "sensitive" {
		t.Fatalf("Tier = %q, want %q (updated to the latest trigger)", b.Tier, "sensitive")
	}
}

func TestUpsertBan_ManualBanRecordsCreatedBy(t *testing.T) {
	db := freshDB(t)
	persist := NewIPBanPersistentRepo(db)
	query := security_query.NewIPBanQueryRepo(db)

	adminID := "6b12ba0b-6b36-4a94-bce4-6ba3615b1f85"
	if err := persist.UpsertBan("203.0.113.7", "manual", "manually banned by admin", time.Now().Add(time.Hour), &adminID); err != nil {
		t.Fatalf("UpsertBan() error = %v", err)
	}
	b, err := query.FindBan("203.0.113.7")
	if err != nil {
		t.Fatalf("FindBan() error = %v", err)
	}
	if b.CreatedBy != adminID {
		t.Fatalf("CreatedBy = %q, want %q", b.CreatedBy, adminID)
	}
}

func TestDeleteBan_ErrorsWhenNoMatchingRow(t *testing.T) {
	db := freshDB(t)
	persist := NewIPBanPersistentRepo(db)

	if err := persist.DeleteBan("203.0.113.7"); err == nil {
		t.Fatal("expected an error unbanning an IP that was never banned")
	}
}

func TestDeleteBan_RemovesExistingRow(t *testing.T) {
	db := freshDB(t)
	persist := NewIPBanPersistentRepo(db)
	query := security_query.NewIPBanQueryRepo(db)

	if err := persist.UpsertBan("203.0.113.7", "global", "ddos rate limit exceeded", time.Now().Add(time.Hour), nil); err != nil {
		t.Fatalf("UpsertBan() error = %v", err)
	}
	if err := persist.DeleteBan("203.0.113.7"); err != nil {
		t.Fatalf("DeleteBan() error = %v", err)
	}
	if _, err := query.FindBan("203.0.113.7"); err != security_query.ErrNotFound {
		t.Fatalf("FindBan() after delete error = %v, want ErrNotFound", err)
	}
}

func TestDeleteExpired_OnlyRemovesPastBans(t *testing.T) {
	db := freshDB(t)
	persist := NewIPBanPersistentRepo(db)
	query := security_query.NewIPBanQueryRepo(db)

	now := time.Now()
	if err := persist.UpsertBan("1.1.1.1", "global", "expired", now.Add(-time.Hour), nil); err != nil {
		t.Fatalf("UpsertBan() expired error = %v", err)
	}
	if err := persist.UpsertBan("2.2.2.2", "global", "still active", now.Add(time.Hour), nil); err != nil {
		t.Fatalf("UpsertBan() active error = %v", err)
	}

	n, err := persist.DeleteExpired(now)
	if err != nil {
		t.Fatalf("DeleteExpired() error = %v", err)
	}
	if n != 1 {
		t.Fatalf("DeleteExpired() removed %d rows, want 1", n)
	}
	if _, err := query.FindBan("1.1.1.1"); err != security_query.ErrNotFound {
		t.Fatalf("expired ban should be gone, FindBan() error = %v", err)
	}
	if _, err := query.FindBan("2.2.2.2"); err != nil {
		t.Fatalf("still-active ban should survive, FindBan() error = %v", err)
	}
}

func TestDeleteExpired_ZeroMatchesIsNotAnError(t *testing.T) {
	db := freshDB(t)
	persist := NewIPBanPersistentRepo(db)

	n, err := persist.DeleteExpired(time.Now())
	if err != nil {
		t.Fatalf("DeleteExpired() with nothing to delete error = %v", err)
	}
	if n != 0 {
		t.Fatalf("DeleteExpired() removed %d rows, want 0", n)
	}
}

func TestIPBanQueryRepo_ListActiveExcludesExpired(t *testing.T) {
	db := freshDB(t)
	persist := NewIPBanPersistentRepo(db)
	query := security_query.NewIPBanQueryRepo(db)

	now := time.Now()
	if err := persist.UpsertBan("1.1.1.1", "global", "expired", now.Add(-time.Hour), nil); err != nil {
		t.Fatalf("UpsertBan() expired error = %v", err)
	}
	if err := persist.UpsertBan("2.2.2.2", "global", "still active", now.Add(time.Hour), nil); err != nil {
		t.Fatalf("UpsertBan() active error = %v", err)
	}

	active, err := query.ListActive(now)
	if err != nil {
		t.Fatalf("ListActive() error = %v", err)
	}
	if len(active) != 1 || active[0].IP != "2.2.2.2" {
		t.Fatalf("ListActive() = %+v, want only 2.2.2.2", active)
	}

	all, err := query.ListBans()
	if err != nil {
		t.Fatalf("ListBans() error = %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("ListBans() returned %d rows, want 2 (includes expired)", len(all))
	}
}
