package oauthserverwiring

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// openTestDB creates an in-memory SQLite with the main-DB users schema
// (id, username, email, organization_id) that mirrors the production
// migration at config/db/migrations/15_02_2026_12_21_22.sql.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec(`
		CREATE TABLE users (
			id              TEXT NOT NULL PRIMARY KEY,
			username        TEXT NOT NULL,
			email           TEXT NOT NULL,
			organization_id TEXT NOT NULL,
			is_active       INTEGER NOT NULL DEFAULT 1,
			must_change_password INTEGER NOT NULL DEFAULT 0
		)`); err != nil {
		t.Fatalf("create users table: %v", err)
	}
	return db
}

func TestLoadClaims_ReturnsEmailAndUsername(t *testing.T) {
	db := openTestDB(t)
	if _, err := db.Exec(
		`INSERT INTO users (id, username, email, organization_id) VALUES (?, ?, ?, ?)`,
		"user-abc", "alice", "alice@example.com", "org-1",
	); err != nil {
		t.Fatalf("insert: %v", err)
	}

	r := &UsersDBClaimsReader{
		Resolver: func(_ string) (*sql.DB, error) { return db, nil },
	}
	claims, err := r.LoadClaims(context.Background(), "org-1", "user-abc", []string{"openid", "profile", "email"})
	if err != nil {
		t.Fatalf("LoadClaims: %v", err)
	}
	if claims["email"] != "alice@example.com" {
		t.Errorf("email: want alice@example.com, got %v", claims["email"])
	}
	if claims["preferred_username"] != "alice" {
		t.Errorf("preferred_username: want alice, got %v", claims["preferred_username"])
	}
	if claims["email_verified"] != true {
		t.Errorf("email_verified: want true, got %v", claims["email_verified"])
	}
}

func TestLoadClaims_UserNotFound_ReturnsNil(t *testing.T) {
	db := openTestDB(t)
	r := &UsersDBClaimsReader{
		Resolver: func(_ string) (*sql.DB, error) { return db, nil },
	}
	claims, err := r.LoadClaims(context.Background(), "org-1", "no-such-user", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claims != nil {
		t.Errorf("want nil claims for missing user, got %v", claims)
	}
}

func TestLoadClaims_ScopesIgnored_AllClaimsReturned(t *testing.T) {
	// Userinfo returns all LoadClaims output regardless of scopes;
	// the scope list is intentionally unused in LoadClaims.
	db := openTestDB(t)
	if _, err := db.Exec(
		`INSERT INTO users (id, username, email, organization_id) VALUES (?, ?, ?, ?)`,
		"user-2", "bob", "bob@example.com", "org-2",
	); err != nil {
		t.Fatalf("insert: %v", err)
	}

	r := &UsersDBClaimsReader{
		Resolver: func(_ string) (*sql.DB, error) { return db, nil },
	}
	// No profile/email scopes — claims still returned.
	claims, err := r.LoadClaims(context.Background(), "org-2", "user-2", []string{"openid", "offline_access"})
	if err != nil {
		t.Fatalf("LoadClaims: %v", err)
	}
	if claims["email"] != "bob@example.com" {
		t.Errorf("email: want bob@example.com, got %v", claims["email"])
	}
	if claims["preferred_username"] != "bob" {
		t.Errorf("preferred_username: want bob, got %v", claims["preferred_username"])
	}
}
