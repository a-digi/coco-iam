package query

import (
	"database/sql"
	"encoding/json"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// adminAclSchema mirrors the production migration exactly.
const adminAclSchema = `
CREATE TABLE admin_users (
	id   TEXT NOT NULL PRIMARY KEY,
	username TEXT NOT NULL,
	email TEXT NOT NULL,
	is_active BOOLEAN NOT NULL DEFAULT TRUE,
	is_super_admin BOOLEAN NOT NULL DEFAULT FALSE,
	must_change_password BOOLEAN NOT NULL DEFAULT FALSE,
	limited_access BOOLEAN NOT NULL DEFAULT TRUE,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE admin_acl (
	id         TEXT NOT NULL CONSTRAINT admin_acl_pk PRIMARY KEY UNIQUE,
	user_id    TEXT NOT NULL,
	roles      JSON NOT NULL,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	is_active  BOOLEAN NOT NULL DEFAULT TRUE
);
CREATE UNIQUE INDEX admin_acl_user_id_uindex ON admin_acl (user_id);
`

func freshDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(adminAclSchema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return db
}

func insertUser(t *testing.T, db *sql.DB, id string) {
	t.Helper()
	_, err := db.Exec(
		`INSERT INTO admin_users (id, username, email) VALUES (?, ?, ?)`,
		id, "user_"+id[:8], id[:8]+"@test.com",
	)
	if err != nil {
		t.Fatalf("insert admin_user: %v", err)
	}
}

// insertAclRaw inserts directly with raw SQL so we can simulate
// problematic rows (NULL created_at, unusual roles encoding).
func insertAclRaw(t *testing.T, db *sql.DB, id, userID, roles string, createdAt *string, isActive bool) {
	t.Helper()
	var err error
	if createdAt == nil {
		_, err = db.Exec(
			`INSERT INTO admin_acl (id, user_id, roles, created_at, is_active) VALUES (?, ?, ?, NULL, ?)`,
			id, userID, roles, isActive,
		)
	} else {
		_, err = db.Exec(
			`INSERT INTO admin_acl (id, user_id, roles, created_at, is_active) VALUES (?, ?, ?, ?, ?)`,
			id, userID, roles, *createdAt, isActive,
		)
	}
	if err != nil {
		t.Fatalf("insertAclRaw: %v", err)
	}
}

// --- AdminUserExists ---

func TestAdminUserExists_ReturnsTrueWhenExists(t *testing.T) {
	db := freshDB(t)
	userID := "aaaaaaaa-0000-0000-0000-000000000001"
	insertUser(t, db, userID)
	repo := NewAdminAclQueryRepo(db)
	got, err := repo.AdminUserExists(userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got {
		t.Error("expected true, got false")
	}
}

func TestAdminUserExists_ReturnsFalseWhenMissing(t *testing.T) {
	db := freshDB(t)
	repo := NewAdminAclQueryRepo(db)
	got, err := repo.AdminUserExists("does-not-exist")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got {
		t.Error("expected false, got true")
	}
}

// --- FindByUserID ---

func TestFindByUserID_ReturnsEmptySliceWhenNone(t *testing.T) {
	db := freshDB(t)
	repo := NewAdminAclQueryRepo(db)
	got, err := repo.FindByUserID("no-such-user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0 entries, got %d", len(got))
	}
}

func TestFindByUserID_ReturnsEntry(t *testing.T) {
	db := freshDB(t)
	userID := "aaaaaaaa-0000-0000-0000-000000000001"
	ts := "2026-01-01 00:00:00"
	insertAclRaw(t, db, "acl-id-1", userID, `["admin:read"]`, &ts, true)

	repo := NewAdminAclQueryRepo(db)
	got, err := repo.FindByUserID(userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(got))
	}
	if got[0].ID != "acl-id-1" {
		t.Errorf("id: got %q, want %q", got[0].ID, "acl-id-1")
	}
	var roles []string
	if err := json.Unmarshal(got[0].Roles, &roles); err != nil {
		t.Fatalf("unmarshal roles: %v", err)
	}
	if len(roles) != 1 || roles[0] != "admin:read" {
		t.Errorf("roles: got %v", roles)
	}
}

// TestFindByUserID_NullCreatedAt reproduces the exact failure on the
// remote server: rows seeded by the migration from user_acl can have
// created_at = NULL even though the column has DEFAULT CURRENT_TIMESTAMP.
func TestFindByUserID_NullCreatedAt(t *testing.T) {
	db := freshDB(t)
	userID := "aaaaaaaa-0000-0000-0000-000000000002"
	insertAclRaw(t, db, "acl-id-2", userID, `[]`, nil, true) // NULL created_at

	repo := NewAdminAclQueryRepo(db)
	got, err := repo.FindByUserID(userID)
	if err != nil {
		t.Fatalf("FindByUserID with NULL created_at must not error, got: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(got))
	}
	if got[0].CreatedAt != "" {
		t.Errorf("CreatedAt should be empty string for NULL, got %q", got[0].CreatedAt)
	}
}

// TestFindByUserID_EmptyRoles verifies that an empty JSON array is
// scanned and re-serialised cleanly.
func TestFindByUserID_EmptyRoles(t *testing.T) {
	db := freshDB(t)
	userID := "aaaaaaaa-0000-0000-0000-000000000003"
	ts := "2026-01-01 00:00:00"
	insertAclRaw(t, db, "acl-id-3", userID, `[]`, &ts, true)

	repo := NewAdminAclQueryRepo(db)
	got, err := repo.FindByUserID(userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var roles []string
	if err := json.Unmarshal(got[0].Roles, &roles); err != nil {
		t.Fatalf("unmarshal empty roles: %v", err)
	}
	if len(roles) != 0 {
		t.Errorf("expected empty roles slice, got %v", roles)
	}
}

// --- FindByID ---

func TestFindByID_ReturnsEntry(t *testing.T) {
	db := freshDB(t)
	ts := "2026-01-01 00:00:00"
	insertAclRaw(t, db, "acl-id-find", "user-find", `["x:y"]`, &ts, true)

	repo := NewAdminAclQueryRepo(db)
	got, err := repo.FindByID("acl-id-find")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.UserID != "user-find" {
		t.Errorf("UserID: got %q, want %q", got.UserID, "user-find")
	}
}

func TestFindByID_NullCreatedAt(t *testing.T) {
	db := freshDB(t)
	insertAclRaw(t, db, "acl-null-ts", "user-null-ts", `["a:b"]`, nil, true)

	repo := NewAdminAclQueryRepo(db)
	got, err := repo.FindByID("acl-null-ts")
	if err != nil {
		t.Fatalf("FindByID with NULL created_at must not error, got: %v", err)
	}
	if got.CreatedAt != "" {
		t.Errorf("CreatedAt should be empty string for NULL, got %q", got.CreatedAt)
	}
}

func TestFindByID_NotFound(t *testing.T) {
	db := freshDB(t)
	repo := NewAdminAclQueryRepo(db)
	_, err := repo.FindByID("ghost")
	if err == nil {
		t.Fatal("expected sql.ErrNoRows, got nil")
	}
}

// --- List ---

func TestList_ReturnsAllEntries(t *testing.T) {
	db := freshDB(t)
	ts := "2026-01-01 00:00:00"
	insertAclRaw(t, db, "acl-a", "user-a", `[]`, &ts, true)
	insertAclRaw(t, db, "acl-b", "user-b", `["s1"]`, nil, false) // NULL created_at

	repo := NewAdminAclQueryRepo(db)
	got, err := repo.List()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(got))
	}
}

func TestList_EmptyTable(t *testing.T) {
	db := freshDB(t)
	repo := NewAdminAclQueryRepo(db)
	got, err := repo.List()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0 entries, got %d", len(got))
	}
}
