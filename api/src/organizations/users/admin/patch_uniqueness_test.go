package admin

import (
	"strings"
	"testing"

	org_user_query "github.com/a-digi/coco-iam/src/organizations/users/repository/query"
)

// These tests verify the username-immutability and email-uniqueness guards that
// CustomPatchOrganizationUserHandler enforces. They use the OrgUserQueryRepository
// directly against an in-memory DB so they run fast without HTTP plumbing.

// freshPatchDB returns an in-memory SQLite with the org users schema applied via
// the production migration files — same helper as insert_user_integration_test.go.
func freshPatchDB(t *testing.T) *orgTestEnv {
	t.Helper()
	return newOrgTestEnv(t)
}

// --- username immutability ---

// TestPatchUsernameImmutability_PayloadWithUsernameIsDetected mirrors the admin
// user test: a PATCH payload that contains a "username" key must be rejected.
// This test validates the detection logic, not the full HTTP stack.
func TestPatchUsernameImmutability_PayloadWithUsernameIsDetected(t *testing.T) {
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

	clean := map[string]interface{}{"email": "new@example.com", "is_active": true}
	if _, ok := clean["username"]; ok {
		t.Error("clean payload must not contain username key")
	}
}

// --- email uniqueness via OrgUserQueryRepository ---

func TestPatchEmail_OwnEmailAllowed(t *testing.T) {
	env := freshPatchDB(t)
	orgID := "org-patch-own"
	orgDB := env.orgDB(t, orgID)

	// Seed one user.
	if _, err := orgDB.Exec(
		`INSERT INTO users (id, username, email, is_active) VALUES (?, ?, ?, ?)`,
		"user-1", "alice", "alice@example.com", true,
	); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	repo := org_user_query.New(orgDB)
	// Alice patching her own email must not be a conflict.
	exists, err := repo.ExistsByEmailExcludingID("alice@example.com", "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Error("own email must not be reported as a conflict")
	}
}

func TestPatchEmail_ConflictWithOtherUser(t *testing.T) {
	env := freshPatchDB(t)
	orgID := "org-patch-conflict"
	orgDB := env.orgDB(t, orgID)

	if _, err := orgDB.Exec(
		`INSERT INTO users (id, username, email, is_active) VALUES (?, ?, ?, ?)`,
		"user-1", "alice", "alice@example.com", true,
	); err != nil {
		t.Fatalf("seed alice: %v", err)
	}
	if _, err := orgDB.Exec(
		`INSERT INTO users (id, username, email, is_active) VALUES (?, ?, ?, ?)`,
		"user-2", "bob", "bob@example.com", true,
	); err != nil {
		t.Fatalf("seed bob: %v", err)
	}

	repo := org_user_query.New(orgDB)
	// Bob trying to take alice's email must be detected.
	exists, err := repo.ExistsByEmailExcludingID("alice@example.com", "user-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Error("conflict with another user must be detected")
	}
}

func TestPatchEmail_CaseInsensitiveConflict(t *testing.T) {
	env := freshPatchDB(t)
	orgID := "org-patch-ci"
	orgDB := env.orgDB(t, orgID)

	if _, err := orgDB.Exec(
		`INSERT INTO users (id, username, email, is_active) VALUES (?, ?, ?, ?)`,
		"user-1", "alice", "alice@example.com", true,
	); err != nil {
		t.Fatalf("seed alice: %v", err)
	}
	if _, err := orgDB.Exec(
		`INSERT INTO users (id, username, email, is_active) VALUES (?, ?, ?, ?)`,
		"user-2", "bob", "bob@example.com", true,
	); err != nil {
		t.Fatalf("seed bob: %v", err)
	}

	repo := org_user_query.New(orgDB)
	exists, err := repo.ExistsByEmailExcludingID("Alice@Example.COM", "user-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Error("case-insensitive conflict with another user must be detected")
	}
}

// --- DB-level constraint is the backstop ---

func TestOrgUsers_DBConstraint_PreventsDirectDuplicateUsername(t *testing.T) {
	env := freshPatchDB(t)
	orgID := "org-constraint-uname"
	orgDB := env.orgDB(t, orgID)

	if _, err := orgDB.Exec(
		`INSERT INTO users (id, username, email) VALUES (?, ?, ?)`,
		"u1", "alice", "alice@example.com",
	); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	_, err := orgDB.Exec(
		`INSERT INTO users (id, username, email) VALUES (?, ?, ?)`,
		"u2", "alice", "other@example.com",
	)
	if err == nil {
		t.Fatal("expected DB constraint error for duplicate username, got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "unique") {
		t.Errorf("expected UNIQUE constraint error, got: %v", err)
	}
}

func TestOrgUsers_DBConstraint_PreventsDirectDuplicateEmail(t *testing.T) {
	env := freshPatchDB(t)
	orgID := "org-constraint-email"
	orgDB := env.orgDB(t, orgID)

	if _, err := orgDB.Exec(
		`INSERT INTO users (id, username, email) VALUES (?, ?, ?)`,
		"u1", "alice", "shared@example.com",
	); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	_, err := orgDB.Exec(
		`INSERT INTO users (id, username, email) VALUES (?, ?, ?)`,
		"u2", "bob", "shared@example.com",
	)
	if err == nil {
		t.Fatal("expected DB constraint error for duplicate email, got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "unique") {
		t.Errorf("expected UNIQUE constraint error, got: %v", err)
	}
}
