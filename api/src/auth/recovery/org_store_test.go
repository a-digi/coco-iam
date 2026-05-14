package recovery

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const orgRecoveriesSchema = `
	CREATE TABLE password_recoveries (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		token_hash TEXT NOT NULL UNIQUE,
		expires_at DATETIME NOT NULL,
		consumed_at DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
`

func freshOrgDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(orgRecoveriesSchema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return db
}

func freshOrgStoreWithDBs(dbs map[string]*sql.DB) *OrgStore {
	ids := make([]string, 0, len(dbs))
	for id := range dbs {
		ids = append(ids, id)
	}
	return &OrgStore{
		orgIDs: func() []string { return ids },
		openDB: func(orgID string) (*sql.DB, error) {
			db, ok := dbs[orgID]
			if !ok {
				return nil, fmt.Errorf("org %s not found", orgID)
			}
			return db, nil
		},
	}
}

func TestOrgStore_InsertAndFindByTokenHash(t *testing.T) {
	db := freshOrgDB(t)
	s := freshOrgStoreWithDBs(map[string]*sql.DB{"org-1": db})
	expires := time.Now().Add(time.Hour).UTC().Truncate(time.Second)

	row := Row{
		UserID:    "user-1",
		TokenHash: "hash-user-1",
		ExpiresAt: expires,
	}
	if err := s.Insert(row, db); err != nil {
		t.Fatalf("insert: %v", err)
	}

	got, gotDB, err := s.FindByTokenHash("hash-user-1")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if gotDB != db {
		t.Error("FindByTokenHash returned wrong DB pointer")
	}
	if got.UserID != "user-1" {
		t.Errorf("UserID: got %q, want user-1", got.UserID)
	}
	if got.ConsumedAt != nil {
		t.Error("ConsumedAt should be nil for fresh row")
	}
}

func TestOrgStore_FindByTokenHash_UserTypeIsAlwaysUser(t *testing.T) {
	db := freshOrgDB(t)
	s := freshOrgStoreWithDBs(map[string]*sql.DB{"org-1": db})
	_ = s.Insert(Row{UserID: "user-2", TokenHash: "hash-user-2", ExpiresAt: time.Now().Add(time.Hour)}, db)

	got, _, err := s.FindByTokenHash("hash-user-2")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got.UserType != UserTypeUser {
		t.Errorf("UserType: got %q, want %q", got.UserType, UserTypeUser)
	}
}

func TestOrgStore_FindByTokenHash_ScansMultipleOrgs(t *testing.T) {
	db1 := freshOrgDB(t)
	db2 := freshOrgDB(t)
	s := freshOrgStoreWithDBs(map[string]*sql.DB{"org-a": db1, "org-b": db2})

	// Insert only into org-b.
	_ = s.Insert(Row{UserID: "user-3", TokenHash: "hash-user-3", ExpiresAt: time.Now().Add(time.Hour)}, db2)

	got, gotDB, err := s.FindByTokenHash("hash-user-3")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if gotDB != db2 {
		t.Error("expected db2 to be returned")
	}
	if got.UserID != "user-3" {
		t.Errorf("UserID: got %q, want user-3", got.UserID)
	}
}

func TestOrgStore_FindByTokenHash_ReturnsErrNotFoundWhenMissing(t *testing.T) {
	db := freshOrgDB(t)
	s := freshOrgStoreWithDBs(map[string]*sql.DB{"org-1": db})

	_, _, err := s.FindByTokenHash("nonexistent")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestOrgStore_ConsumeByID_MarksConsumedAndDeletesSiblings(t *testing.T) {
	db := freshOrgDB(t)
	s := freshOrgStoreWithDBs(map[string]*sql.DB{"org-1": db})
	expires := time.Now().Add(time.Hour)

	_ = s.Insert(Row{ID: "rec-1", UserID: "user-4", TokenHash: "h1", ExpiresAt: expires}, db)
	_ = s.Insert(Row{ID: "rec-2", UserID: "user-4", TokenHash: "h2", ExpiresAt: expires}, db)

	if err := s.ConsumeByID("rec-1", db); err != nil {
		t.Fatalf("consume: %v", err)
	}

	var consumed sql.NullString
	if err := db.QueryRow(`SELECT consumed_at FROM password_recoveries WHERE id = ?`, "rec-1").Scan(&consumed); err != nil {
		t.Fatalf("lookup rec-1: %v", err)
	}
	if !consumed.Valid {
		t.Error("rec-1 should have consumed_at set")
	}

	var remaining int
	if err := db.QueryRow(`SELECT COUNT(*) FROM password_recoveries WHERE id = ?`, "rec-2").Scan(&remaining); err != nil {
		t.Fatalf("count rec-2: %v", err)
	}
	if remaining != 0 {
		t.Errorf("sibling rec-2 should be deleted, got %d rows", remaining)
	}
}

func TestOrgStore_ConsumeByID_ReturnsErrAlreadyUsedWhenConsumed(t *testing.T) {
	db := freshOrgDB(t)
	s := freshOrgStoreWithDBs(map[string]*sql.DB{"org-1": db})
	_ = s.Insert(Row{ID: "rec-3", UserID: "user-5", TokenHash: "h3", ExpiresAt: time.Now().Add(time.Hour)}, db)
	_ = s.ConsumeByID("rec-3", db)

	if err := s.ConsumeByID("rec-3", db); err != ErrAlreadyUsed {
		t.Errorf("expected ErrAlreadyUsed, got %v", err)
	}
}

func TestOrgStore_DeletePendingForUser_RemovesOnlyUnconsumed(t *testing.T) {
	db := freshOrgDB(t)
	s := freshOrgStoreWithDBs(map[string]*sql.DB{"org-1": db})
	expires := time.Now().Add(time.Hour)

	_ = s.Insert(Row{ID: "rec-4", UserID: "user-6", TokenHash: "h4", ExpiresAt: expires}, db)
	_ = s.Insert(Row{ID: "rec-5", UserID: "user-6", TokenHash: "h5", ExpiresAt: expires}, db)
	_ = s.ConsumeByID("rec-4", db)

	if err := s.DeletePendingForUser("user-6", db); err != nil {
		t.Fatalf("delete pending: %v", err)
	}

	var count int
	_ = db.QueryRow(`SELECT COUNT(*) FROM password_recoveries WHERE user_id = ?`, "user-6").Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 consumed row to survive, got %d", count)
	}
}

func TestOrgStore_LatestPendingForUser_ReturnsNewest(t *testing.T) {
	db := freshOrgDB(t)
	s := freshOrgStoreWithDBs(map[string]*sql.DB{"org-1": db})
	expires := time.Now().Add(time.Hour)

	_ = s.Insert(Row{ID: "rec-6", UserID: "user-7", TokenHash: "h6", ExpiresAt: expires}, db)
	_ = s.Insert(Row{ID: "rec-7", UserID: "user-7", TokenHash: "h7", ExpiresAt: expires}, db)

	latest, err := s.LatestPendingForUser("user-7", db)
	if err != nil {
		t.Fatalf("latest: %v", err)
	}
	if latest == nil {
		t.Fatal("expected a row, got nil")
	}
	if latest.UserType != UserTypeUser {
		t.Errorf("UserType: got %q, want %q", latest.UserType, UserTypeUser)
	}
}

func TestOrgStore_LatestPendingForUser_ReturnsNilWhenNone(t *testing.T) {
	db := freshOrgDB(t)
	s := freshOrgStoreWithDBs(map[string]*sql.DB{"org-1": db})

	latest, err := s.LatestPendingForUser("no-such-user", db)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if latest != nil {
		t.Errorf("expected nil, got %+v", latest)
	}
}
