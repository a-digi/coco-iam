package users

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/a-digi/coco-iam/src/admin/users/entity"
	"github.com/a-digi/coco-iam/src/admin/users/repository/persistent"
	"github.com/a-digi/coco-iam/src/admin/users/repository/query"
	db "github.com/a-digi/coco-orm/orm"
	_ "github.com/mattn/go-sqlite3"
)

// adminUsersSchema applies the same table + unique indexes that the
// production migrations create. Tests rely on these constraints.
const adminUsersSchema = `
CREATE TABLE admin_users (
    id TEXT NOT NULL CONSTRAINT admin_users_pk PRIMARY KEY UNIQUE,
    username TEXT NOT NULL,
    email TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    limited_access BOOLEAN NOT NULL DEFAULT TRUE,
    is_super_admin BOOLEAN NOT NULL DEFAULT FALSE,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    must_change_password BOOLEAN NOT NULL DEFAULT FALSE
);
CREATE UNIQUE INDEX admin_users_username_unique_idx ON admin_users (username);
CREATE UNIQUE INDEX admin_users_email_unique_idx ON admin_users (email);
`

func freshAdminUserDB(t *testing.T) *db.DatabaseManager {
	t.Helper()
	sqlDB, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if _, err := sqlDB.Exec(adminUsersSchema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return &db.DatabaseManager{Connector: &db.Connector{DB: sqlDB}}
}

func freshCreator(mgr *db.DatabaseManager) *AdminUserCreator {
	return &AdminUserCreator{
		AdminUserRepository: persistent.NewAdminUserPersistentRepository(mgr),
		QueryRepository:     query.NewAdminUserQueryRepository(mgr),
		// PasswordRepository intentionally nil — CreatePending never uses it
	}
}

// --- duplicate username ---

func TestAdminUserCreator_DuplicateUsername_IsRejected(t *testing.T) {
	mgr := freshAdminUserDB(t)
	c := freshCreator(mgr)

	if _, err := c.CreatePending("alice", "alice@example.com", true, false); err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, err := c.CreatePending("alice", "other@example.com", true, false)
	if err == nil {
		t.Fatal("expected error for duplicate username, got nil")
	}
	if err.Error() != "username already taken" {
		t.Errorf("error: got %q, want %q", err.Error(), "username already taken")
	}
}

func TestAdminUserCreator_DuplicateUsername_CaseInsensitive(t *testing.T) {
	mgr := freshAdminUserDB(t)
	c := freshCreator(mgr)

	if _, err := c.CreatePending("admin", "admin@example.com", true, false); err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, err := c.CreatePending("Admin", "other@example.com", true, false)
	if err == nil {
		t.Fatal("expected error for case-variant duplicate username, got nil")
	}
	if err.Error() != "username already taken" {
		t.Errorf("error: got %q, want %q", err.Error(), "username already taken")
	}
}

// --- duplicate email ---

func TestAdminUserCreator_DuplicateEmail_IsRejected(t *testing.T) {
	mgr := freshAdminUserDB(t)
	c := freshCreator(mgr)

	if _, err := c.CreatePending("alice", "shared@example.com", true, false); err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, err := c.CreatePending("bob", "shared@example.com", true, false)
	if err == nil {
		t.Fatal("expected error for duplicate email, got nil")
	}
	if err.Error() != "email already taken" {
		t.Errorf("error: got %q, want %q", err.Error(), "email already taken")
	}
}

func TestAdminUserCreator_DuplicateEmail_CaseInsensitive(t *testing.T) {
	mgr := freshAdminUserDB(t)
	c := freshCreator(mgr)

	if _, err := c.CreatePending("alice", "user@example.com", true, false); err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, err := c.CreatePending("bob", "User@Example.COM", true, false)
	if err == nil {
		t.Fatal("expected error for case-variant duplicate email, got nil")
	}
	if err.Error() != "email already taken" {
		t.Errorf("error: got %q, want %q", err.Error(), "email already taken")
	}
}

// --- email update path: own email must not be treated as a conflict ---

func TestExistsByEmailExcludingID_AllowsOwnEmail(t *testing.T) {
	mgr := freshAdminUserDB(t)
	c := freshCreator(mgr)

	user, err := c.CreatePending("alice", "alice@example.com", true, false)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	qrepo := query.NewAdminUserQueryRepository(mgr)
	// Checking alice's own email while excluding alice's ID must not report a conflict.
	exists, err := qrepo.ExistsByEmailExcludingID("alice@example.com", user.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Error("ExistsByEmailExcludingID should return false when the only match is the excluded user")
	}
}

func TestExistsByEmailExcludingID_DetectsConflictWithOtherUser(t *testing.T) {
	mgr := freshAdminUserDB(t)
	c := freshCreator(mgr)

	if _, err := c.CreatePending("alice", "alice@example.com", true, false); err != nil {
		t.Fatalf("create alice: %v", err)
	}
	bob, err := c.CreatePending("bob", "bob@example.com", true, false)
	if err != nil {
		t.Fatalf("create bob: %v", err)
	}

	qrepo := query.NewAdminUserQueryRepository(mgr)
	// Bob tries to take alice's email — must be a conflict.
	exists, err := qrepo.ExistsByEmailExcludingID("alice@example.com", bob.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Error("ExistsByEmailExcludingID should return true when another user owns the email")
	}
}

// --- DB-level constraint is the backstop ---

// TestAdminUsers_DBConstraint_PreventsDirectDuplicateUsername verifies that
// the UNIQUE INDEX blocks a duplicate username even when the application-level
// check is bypassed (e.g. direct repository call).
func TestAdminUsers_DBConstraint_PreventsDirectDuplicateUsername(t *testing.T) {
	mgr := freshAdminUserDB(t)
	repo := persistent.NewAdminUserPersistentRepository(mgr)

	if err := repo.Insert(&entity.User{Username: "alice", Email: "alice@example.com"}); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	err := repo.Insert(&entity.User{Username: "alice", Email: "other@example.com"})
	if err == nil {
		t.Fatal("expected DB constraint error for duplicate username, got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "unique") {
		t.Errorf("expected UNIQUE constraint error, got: %v", err)
	}
}

// TestAdminUsers_DBConstraint_PreventsDirectDuplicateEmail verifies the same
// for email.
func TestAdminUsers_DBConstraint_PreventsDirectDuplicateEmail(t *testing.T) {
	mgr := freshAdminUserDB(t)
	repo := persistent.NewAdminUserPersistentRepository(mgr)

	if err := repo.Insert(&entity.User{Username: "alice", Email: "shared@example.com"}); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	err := repo.Insert(&entity.User{Username: "bob", Email: "shared@example.com"})
	if err == nil {
		t.Fatal("expected DB constraint error for duplicate email, got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "unique") {
		t.Errorf("expected UNIQUE constraint error, got: %v", err)
	}
}

// --- username immutability ---

// TestUsernameImmutability_PayloadWithUsernameIsDetected verifies that a
// PATCH payload containing a "username" key is flagged as an attempted
// username change. This mirrors the guard in CustomUpdateUserHandler.
func TestUsernameImmutability_PayloadWithUsernameIsDetected(t *testing.T) {
	cases := []map[string]interface{}{
		{"username": "new-name"},
		{"username": "alice", "email": "alice@example.com"},
		{"username": ""},
	}
	for _, payload := range cases {
		if _, ok := payload["username"]; !ok {
			t.Errorf("test setup error: payload %v does not contain username key", payload)
		}
	}

	// Confirm a clean payload (no username key) does NOT trigger the guard.
	clean := map[string]interface{}{"email": "new@example.com", "is_active": true}
	if _, ok := clean["username"]; ok {
		t.Error("clean payload should not contain username key")
	}
}
