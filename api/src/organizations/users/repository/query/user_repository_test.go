package query

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

const orgUsersSchema = `
CREATE TABLE users (
    id TEXT NOT NULL CONSTRAINT users_pk PRIMARY KEY UNIQUE,
    username TEXT NOT NULL,
    email TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    must_change_password BOOLEAN NOT NULL DEFAULT FALSE
);
CREATE UNIQUE INDEX users_email_unique_idx ON users (email);
CREATE UNIQUE INDEX users_username_unique_idx ON users (username);
`

func freshOrgUserDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(orgUsersSchema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return db
}

func insertOrgUser(t *testing.T, db *sql.DB, id, username, email string) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO users (id, username, email) VALUES (?, ?, ?)`,
		id, username, email,
	); err != nil {
		t.Fatalf("insert user: %v", err)
	}
}

// --- ExistsByUsername ---

func TestExistsByUsername_ReturnsFalseWhenEmpty(t *testing.T) {
	repo := New(freshOrgUserDB(t))
	exists, err := repo.ExistsByUsername("alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Error("want false for empty table, got true")
	}
}

func TestExistsByUsername_ReturnsTrueForExact(t *testing.T) {
	db := freshOrgUserDB(t)
	insertOrgUser(t, db, "id-1", "alice", "alice@example.com")
	repo := New(db)

	exists, err := repo.ExistsByUsername("alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Error("want true for existing username, got false")
	}
}

func TestExistsByUsername_CaseInsensitive(t *testing.T) {
	db := freshOrgUserDB(t)
	insertOrgUser(t, db, "id-1", "alice", "alice@example.com")
	repo := New(db)

	for _, variant := range []string{"Alice", "ALICE", "aLiCe"} {
		exists, err := repo.ExistsByUsername(variant)
		if err != nil {
			t.Fatalf("ExistsByUsername(%q): unexpected error: %v", variant, err)
		}
		if !exists {
			t.Errorf("ExistsByUsername(%q): want true, got false", variant)
		}
	}
}

func TestExistsByUsername_ReturnsFalseForDifferentUser(t *testing.T) {
	db := freshOrgUserDB(t)
	insertOrgUser(t, db, "id-1", "alice", "alice@example.com")
	repo := New(db)

	exists, err := repo.ExistsByUsername("bob")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Error("want false for different username, got true")
	}
}

// --- ExistsByEmailExcludingID (create path: excludeID="") ---

func TestExistsByEmailExcludingID_ReturnsFalseWhenEmpty(t *testing.T) {
	repo := New(freshOrgUserDB(t))
	exists, err := repo.ExistsByEmailExcludingID("user@example.com", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Error("want false for empty table, got true")
	}
}

func TestExistsByEmailExcludingID_ReturnsTrueForExact(t *testing.T) {
	db := freshOrgUserDB(t)
	insertOrgUser(t, db, "id-1", "alice", "alice@example.com")
	repo := New(db)

	exists, err := repo.ExistsByEmailExcludingID("alice@example.com", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Error("want true for existing email, got false")
	}
}

func TestExistsByEmailExcludingID_CaseInsensitive(t *testing.T) {
	db := freshOrgUserDB(t)
	insertOrgUser(t, db, "id-1", "alice", "alice@example.com")
	repo := New(db)

	for _, variant := range []string{"Alice@Example.COM", "ALICE@EXAMPLE.COM", "Alice@example.com"} {
		exists, err := repo.ExistsByEmailExcludingID(variant, "")
		if err != nil {
			t.Fatalf("ExistsByEmailExcludingID(%q): unexpected error: %v", variant, err)
		}
		if !exists {
			t.Errorf("ExistsByEmailExcludingID(%q): want true, got false", variant)
		}
	}
}

// --- ExistsByEmailExcludingID (update path: excludeID set) ---

func TestExistsByEmailExcludingID_AllowsOwnEmail(t *testing.T) {
	db := freshOrgUserDB(t)
	insertOrgUser(t, db, "id-1", "alice", "alice@example.com")
	repo := New(db)

	exists, err := repo.ExistsByEmailExcludingID("alice@example.com", "id-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Error("ExistsByEmailExcludingID should return false when the only match is the excluded user")
	}
}

func TestExistsByEmailExcludingID_DetectsConflictWithOtherUser(t *testing.T) {
	db := freshOrgUserDB(t)
	insertOrgUser(t, db, "id-1", "alice", "alice@example.com")
	insertOrgUser(t, db, "id-2", "bob", "bob@example.com")
	repo := New(db)

	// bob tries to take alice's email
	exists, err := repo.ExistsByEmailExcludingID("alice@example.com", "id-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Error("ExistsByEmailExcludingID should return true when another user owns the email")
	}
}

func TestExistsByEmailExcludingID_CaseInsensitiveExcluding(t *testing.T) {
	db := freshOrgUserDB(t)
	insertOrgUser(t, db, "id-1", "alice", "alice@example.com")
	insertOrgUser(t, db, "id-2", "bob", "bob@example.com")
	repo := New(db)

	// bob tries to take alice's email with different casing
	exists, err := repo.ExistsByEmailExcludingID("Alice@Example.COM", "id-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Error("ExistsByEmailExcludingID should detect case-insensitive conflict with another user")
	}
}
