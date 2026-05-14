package recovery

import (
	"database/sql"
	"testing"
	"time"

	db "github.com/a-digi/coco-orm/orm"
	_ "github.com/mattn/go-sqlite3"
)

const adminRecoveriesSchema = `
	CREATE TABLE admin_password_recoveries (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		token_hash TEXT NOT NULL UNIQUE,
		expires_at DATETIME NOT NULL,
		consumed_at DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
`

func freshAdminStore(t *testing.T) *AdminStore {
	t.Helper()
	sqlDB, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if _, err := sqlDB.Exec(adminRecoveriesSchema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return &AdminStore{db: sqlDB}
}

func freshAdminStoreFromManager(t *testing.T) *AdminStore {
	t.Helper()
	sqlDB, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if _, err := sqlDB.Exec(adminRecoveriesSchema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return NewAdminStore(&db.DatabaseManager{Connector: &db.Connector{DB: sqlDB}})
}

func TestAdminStore_InsertAndFindByTokenHash(t *testing.T) {
	s := freshAdminStore(t)
	expires := time.Now().Add(time.Hour).UTC().Truncate(time.Second)

	row := Row{
		UserID:    "admin-1",
		TokenHash: "hash-admin-1",
		ExpiresAt: expires,
	}
	if err := s.Insert(row); err != nil {
		t.Fatalf("insert: %v", err)
	}

	got, err := s.FindByTokenHash("hash-admin-1")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got.UserID != "admin-1" {
		t.Errorf("UserID: got %q, want admin-1", got.UserID)
	}
	if got.TokenHash != "hash-admin-1" {
		t.Errorf("TokenHash: got %q, want hash-admin-1", got.TokenHash)
	}
	if got.ConsumedAt != nil {
		t.Error("ConsumedAt should be nil for fresh row")
	}
}

func TestAdminStore_FindByTokenHash_UserTypeIsAlwaysAdmin(t *testing.T) {
	s := freshAdminStore(t)
	_ = s.Insert(Row{UserID: "admin-2", TokenHash: "hash-admin-2", ExpiresAt: time.Now().Add(time.Hour)})

	got, err := s.FindByTokenHash("hash-admin-2")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got.UserType != UserTypeAdmin {
		t.Errorf("UserType: got %q, want %q", got.UserType, UserTypeAdmin)
	}
}

func TestAdminStore_FindByTokenHash_ReturnsErrNotFoundForUnknown(t *testing.T) {
	s := freshAdminStore(t)
	_, err := s.FindByTokenHash("nonexistent")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestAdminStore_ConsumeByID_MarksConsumedAndDeletesSiblings(t *testing.T) {
	s := freshAdminStore(t)
	expires := time.Now().Add(time.Hour)

	_ = s.Insert(Row{ID: "rec-1", UserID: "admin-3", TokenHash: "h1", ExpiresAt: expires})
	_ = s.Insert(Row{ID: "rec-2", UserID: "admin-3", TokenHash: "h2", ExpiresAt: expires})

	if err := s.ConsumeByID("rec-1"); err != nil {
		t.Fatalf("consume: %v", err)
	}

	var consumed sql.NullString
	if err := s.db.QueryRow(`SELECT consumed_at FROM admin_password_recoveries WHERE id = ?`, "rec-1").Scan(&consumed); err != nil {
		t.Fatalf("lookup rec-1: %v", err)
	}
	if !consumed.Valid {
		t.Error("rec-1 should have consumed_at set")
	}

	var remaining int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM admin_password_recoveries WHERE id = ?`, "rec-2").Scan(&remaining); err != nil {
		t.Fatalf("count rec-2: %v", err)
	}
	if remaining != 0 {
		t.Errorf("sibling rec-2 should be deleted, got %d rows", remaining)
	}
}

func TestAdminStore_ConsumeByID_ReturnsErrAlreadyUsedWhenConsumed(t *testing.T) {
	s := freshAdminStore(t)
	_ = s.Insert(Row{ID: "rec-3", UserID: "admin-4", TokenHash: "h3", ExpiresAt: time.Now().Add(time.Hour)})
	_ = s.ConsumeByID("rec-3")

	if err := s.ConsumeByID("rec-3"); err != ErrAlreadyUsed {
		t.Errorf("expected ErrAlreadyUsed, got %v", err)
	}
}

func TestAdminStore_DeletePendingForUser_RemovesOnlyUnconsumed(t *testing.T) {
	s := freshAdminStore(t)
	expires := time.Now().Add(time.Hour)

	_ = s.Insert(Row{ID: "rec-4", UserID: "admin-5", TokenHash: "h4", ExpiresAt: expires})
	_ = s.Insert(Row{ID: "rec-5", UserID: "admin-5", TokenHash: "h5", ExpiresAt: expires})
	// consume rec-4 so it should survive DeletePending
	_ = s.ConsumeByID("rec-4")

	if err := s.DeletePendingForUser("admin-5"); err != nil {
		t.Fatalf("delete pending: %v", err)
	}

	var count int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM admin_password_recoveries WHERE user_id = ?`, "admin-5").Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 consumed row to survive, got %d", count)
	}
}

func TestAdminStore_LatestPendingForUser_ReturnsNewest(t *testing.T) {
	s := freshAdminStore(t)
	expires := time.Now().Add(time.Hour)

	_ = s.Insert(Row{ID: "rec-6", UserID: "admin-6", TokenHash: "h6", ExpiresAt: expires})
	_ = s.Insert(Row{ID: "rec-7", UserID: "admin-6", TokenHash: "h7", ExpiresAt: expires})

	latest, err := s.LatestPendingForUser("admin-6")
	if err != nil {
		t.Fatalf("latest: %v", err)
	}
	if latest == nil {
		t.Fatal("expected a row, got nil")
	}
	if latest.UserType != UserTypeAdmin {
		t.Errorf("UserType: got %q, want %q", latest.UserType, UserTypeAdmin)
	}
}

func TestAdminStore_LatestPendingForUser_ReturnsNilWhenNone(t *testing.T) {
	s := freshAdminStore(t)
	latest, err := s.LatestPendingForUser("no-such-admin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if latest != nil {
		t.Errorf("expected nil, got %+v", latest)
	}
}

func TestAdminStore_NewAdminStore_ConstructorWorks(t *testing.T) {
	s := freshAdminStoreFromManager(t)
	if s == nil || s.db == nil {
		t.Fatal("NewAdminStore returned nil or empty store")
	}
}
