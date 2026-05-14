package admin

import (
	"database/sql"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/a-digi/coco-iam/src/organizations/users/dbregistry"
	"github.com/a-digi/coco-iam/src/orgrouter"
	users_entity "github.com/a-digi/coco-iam/src/organizations/users/entity"
	_ "github.com/mattn/go-sqlite3"
)

// orgUserMigrationsPath returns the absolute path to the per-org user
// migration files regardless of where `go test` is invoked from.
func orgUserMigrationsPath() string {
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), "../../../../config/db/org_user_migrations")
}

type orgTestEnv struct {
	mainDB   *sql.DB
	registry *dbregistry.OrgUserDBRegistry
}

// newOrgTestEnv creates an in-memory main DB (with routing tables) and a
// real on-disk OrgUserDBRegistry rooted at t.TempDir(). Migrations are
// applied from the production org_user_migrations directory so any schema
// drift is caught immediately.
func newOrgTestEnv(t *testing.T) *orgTestEnv {
	t.Helper()

	mainDB, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open main db: %v", err)
	}
	t.Cleanup(func() { mainDB.Close() })

	if _, err := mainDB.Exec(`
		CREATE TABLE user_org_index (
			user_id TEXT NOT NULL PRIMARY KEY,
			org_id  TEXT NOT NULL
		);
		CREATE TABLE user_email_org_index (
			email   TEXT NOT NULL,
			org_id  TEXT NOT NULL,
			user_id TEXT NOT NULL,
			PRIMARY KEY (email, org_id)
		);
	`); err != nil {
		t.Fatalf("create routing tables: %v", err)
	}

	reg := dbregistry.New(t.TempDir(), orgUserMigrationsPath())
	return &orgTestEnv{mainDB: mainDB, registry: reg}
}

// orgDB opens (and migrates) the per-org users DB for orgID.
func (e *orgTestEnv) orgDB(t *testing.T, orgID string) *sql.DB {
	t.Helper()
	mgr, err := e.registry.For(orgID)
	if err != nil {
		t.Fatalf("registry.For(%q): %v", orgID, err)
	}
	return mgr.Connector.DB
}

// --- tests ---

func TestInsertUser_WritesToOrgDB(t *testing.T) {
	env := newOrgTestEnv(t)
	orgID := "org-test-1"
	db := env.orgDB(t, orgID)

	u := &users_entity.User{
		Username:       "alice",
		Email:          "alice@example.com",
		OrganizationID: orgID,
		IsActive:       true,
	}
	if err := insertUser(env.mainDB, db, u); err != nil {
		t.Fatalf("insertUser: %v", err)
	}

	var gotUsername, gotEmail string
	var gotActive bool
	err := db.QueryRow(
		`SELECT username, email, is_active FROM users WHERE id = ?`, u.ID,
	).Scan(&gotUsername, &gotEmail, &gotActive)
	if err != nil {
		t.Fatalf("SELECT from org db: %v", err)
	}
	if gotUsername != "alice" {
		t.Errorf("username: want alice, got %q", gotUsername)
	}
	if gotEmail != "alice@example.com" {
		t.Errorf("email: want alice@example.com, got %q", gotEmail)
	}
	if !gotActive {
		t.Error("is_active: want true, got false")
	}
}

func TestInsertUser_UserNotInMainDB(t *testing.T) {
	env := newOrgTestEnv(t)
	orgID := "org-test-2"
	orgDB := env.orgDB(t, orgID)

	u := &users_entity.User{
		Username:       "bob",
		Email:          "bob@example.com",
		OrganizationID: orgID,
		IsActive:       true,
	}
	if err := insertUser(env.mainDB, orgDB, u); err != nil {
		t.Fatalf("insertUser: %v", err)
	}

	// The main DB has no `users` table — querying it must error, not
	// silently return a row. This confirms org users are not written there.
	var count int
	err := env.mainDB.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count)
	if err == nil {
		t.Errorf("expected error querying users in main DB, got count=%d", count)
	}
}

func TestInsertUser_OnlyInOrgDB(t *testing.T) {
	env := newOrgTestEnv(t)
	orgID := "org-test-3"
	orgDB := env.orgDB(t, orgID)

	u := &users_entity.User{
		Username:       "carol",
		Email:          "carol@example.com",
		OrganizationID: orgID,
		IsActive:       true,
	}
	if err := insertUser(env.mainDB, orgDB, u); err != nil {
		t.Fatalf("insertUser: %v", err)
	}

	var username string
	if err := orgDB.QueryRow(
		`SELECT username FROM users WHERE id = ?`, u.ID,
	).Scan(&username); err != nil {
		t.Fatalf("user not found in org db: %v", err)
	}
	if username != "carol" {
		t.Errorf("username: want carol, got %q", username)
	}
}

func TestInsertUser_OrgDBForRoundTrip(t *testing.T) {
	env := newOrgTestEnv(t)
	orgID := "org-test-4"
	orgDB := env.orgDB(t, orgID)

	u := &users_entity.User{
		Username:       "dave",
		Email:          "dave@example.com",
		OrganizationID: orgID,
		IsActive:       true,
	}
	if err := insertUser(env.mainDB, orgDB, u); err != nil {
		t.Fatalf("insertUser: %v", err)
	}

	resolvedDB, _, err := orgrouter.OrgDBFor(env.registry, u.ID)
	if err != nil {
		t.Fatalf("OrgDBFor: %v", err)
	}

	var username string
	if err := resolvedDB.QueryRow(
		`SELECT username FROM users WHERE id = ?`, u.ID,
	).Scan(&username); err != nil {
		t.Fatalf("SELECT via resolved db: %v", err)
	}
	if username != "dave" {
		t.Errorf("username via OrgDBFor: want dave, got %q", username)
	}
}

func TestInsertUser_TwoOrgsIsolated(t *testing.T) {
	env := newOrgTestEnv(t)

	orgADB := env.orgDB(t, "org-a")
	orgBDB := env.orgDB(t, "org-b")

	uA := &users_entity.User{Username: "alice", Email: "alice@a.com", OrganizationID: "org-a", IsActive: true}
	uB := &users_entity.User{Username: "bob", Email: "bob@b.com", OrganizationID: "org-b", IsActive: true}

	if err := insertUser(env.mainDB, orgADB, uA); err != nil {
		t.Fatalf("insertUser org-a: %v", err)
	}
	if err := insertUser(env.mainDB, orgBDB, uB); err != nil {
		t.Fatalf("insertUser org-b: %v", err)
	}

	// org-A's DB must not contain org-B's user.
	var count int
	if err := orgADB.QueryRow(
		`SELECT COUNT(*) FROM users WHERE id = ?`, uB.ID,
	).Scan(&count); err != nil {
		t.Fatalf("count in org-a db: %v", err)
	}
	if count != 0 {
		t.Errorf("org-B user leaked into org-A db (count=%d)", count)
	}

	// org-B's DB must not contain org-A's user.
	if err := orgBDB.QueryRow(
		`SELECT COUNT(*) FROM users WHERE id = ?`, uA.ID,
	).Scan(&count); err != nil {
		t.Fatalf("count in org-b db: %v", err)
	}
	if count != 0 {
		t.Errorf("org-A user leaked into org-B db (count=%d)", count)
	}
}

func TestInsertUser_DuplicateEmailSameOrg_Fails(t *testing.T) {
	env := newOrgTestEnv(t)
	orgID := "org-test-dup"
	orgDB := env.orgDB(t, orgID)

	u1 := &users_entity.User{Username: "eve", Email: "eve@example.com", OrganizationID: orgID, IsActive: true}
	if err := insertUser(env.mainDB, orgDB, u1); err != nil {
		t.Fatalf("first insertUser: %v", err)
	}

	u2 := &users_entity.User{Username: "eve2", Email: "eve@example.com", OrganizationID: orgID, IsActive: true}
	if err := insertUser(env.mainDB, orgDB, u2); err == nil {
		t.Error("expected UNIQUE constraint error for duplicate email, got nil")
	}
}

func TestInsertUser_DuplicateUsernameSameOrg_Fails(t *testing.T) {
	env := newOrgTestEnv(t)
	orgID := "org-test-dup-uname"
	orgDB := env.orgDB(t, orgID)

	u1 := &users_entity.User{Username: "grace", Email: "grace@example.com", OrganizationID: orgID, IsActive: true}
	if err := insertUser(env.mainDB, orgDB, u1); err != nil {
		t.Fatalf("first insertUser: %v", err)
	}

	u2 := &users_entity.User{Username: "grace", Email: "grace2@example.com", OrganizationID: orgID, IsActive: true}
	if err := insertUser(env.mainDB, orgDB, u2); err == nil {
		t.Error("expected UNIQUE constraint error for duplicate username, got nil")
	}
}

func TestInsertUser_SameUsernameInDifferentOrgs_Allowed(t *testing.T) {
	env := newOrgTestEnv(t)

	orgADB := env.orgDB(t, "org-uname-a")
	orgBDB := env.orgDB(t, "org-uname-b")

	uA := &users_entity.User{Username: "henry", Email: "henry@a.com", OrganizationID: "org-uname-a", IsActive: true}
	uB := &users_entity.User{Username: "henry", Email: "henry@b.com", OrganizationID: "org-uname-b", IsActive: true}

	if err := insertUser(env.mainDB, orgADB, uA); err != nil {
		t.Fatalf("insertUser org-a: %v", err)
	}
	if err := insertUser(env.mainDB, orgBDB, uB); err != nil {
		t.Fatalf("insertUser org-b: same username in different org must be allowed: %v", err)
	}
}

func TestInsertUser_SameEmailInDifferentOrgs_Allowed(t *testing.T) {
	env := newOrgTestEnv(t)

	orgADB := env.orgDB(t, "org-email-a")
	orgBDB := env.orgDB(t, "org-email-b")

	uA := &users_entity.User{Username: "irene", Email: "shared@example.com", OrganizationID: "org-email-a", IsActive: true}
	uB := &users_entity.User{Username: "irene2", Email: "shared@example.com", OrganizationID: "org-email-b", IsActive: true}

	if err := insertUser(env.mainDB, orgADB, uA); err != nil {
		t.Fatalf("insertUser org-a: %v", err)
	}
	if err := insertUser(env.mainDB, orgBDB, uB); err != nil {
		t.Fatalf("insertUser org-b: same email in different org must be allowed: %v", err)
	}
}

func TestInsertUser_GeneratesIDIfEmpty(t *testing.T) {
	env := newOrgTestEnv(t)
	orgID := "org-test-id"
	orgDB := env.orgDB(t, orgID)

	u := &users_entity.User{
		Username:       "frank",
		Email:          "frank@example.com",
		OrganizationID: orgID,
		IsActive:       true,
		// ID intentionally empty
	}
	if err := insertUser(env.mainDB, orgDB, u); err != nil {
		t.Fatalf("insertUser: %v", err)
	}

	if u.ID == "" {
		t.Error("expected non-empty ID after insertUser")
	}
	// UUID format: 8-4-4-4-12 hex chars
	if len(u.ID) != 36 {
		t.Errorf("ID length: want 36, got %d (%q)", len(u.ID), u.ID)
	}
}
