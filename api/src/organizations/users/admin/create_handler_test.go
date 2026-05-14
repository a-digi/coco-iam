package admin

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/a-digi/coco-iam/src/organizations/users/dbregistry"
	db "github.com/a-digi/coco-orm/orm"
	_ "github.com/mattn/go-sqlite3"
)

// freshMainDB creates an in-memory SQLite with just the organization table
// (the global DB that resolveRedirectTarget uses for org slug lookup).
func freshMainDB(t *testing.T) *db.DatabaseManager {
	t.Helper()
	sqlDB, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if _, err := sqlDB.Exec(`
		CREATE TABLE organization (
			id TEXT NOT NULL PRIMARY KEY,
			organization_id TEXT NOT NULL,
			title TEXT NOT NULL DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			is_active BOOLEAN NOT NULL DEFAULT 1
		);
	`); err != nil {
		t.Fatalf("create main schema: %v", err)
	}
	return &db.DatabaseManager{Connector: &db.Connector{DB: sqlDB}}
}

// freshOrgRegistry returns a registry backed by a temp directory and the
// *sql.DB handle for the named org. The DB is pre-seeded with the workspace
// and applications schemas.
func freshOrgRegistry(t *testing.T, orgID string) (*dbregistry.OrgUserDBRegistry, *sql.DB) {
	t.Helper()
	baseDir := t.TempDir()
	migrationsDir := t.TempDir() // empty dir — no migrations to apply

	orgDir := filepath.Join(baseDir, "organization", orgID)
	if err := os.MkdirAll(orgDir, 0o755); err != nil {
		t.Fatalf("mkdir org dir: %v", err)
	}
	dbPath := filepath.Join(orgDir, "users.db")
	seedDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open seed db: %v", err)
	}
	if _, err := seedDB.Exec(`
		CREATE TABLE IF NOT EXISTS workspace (
			id TEXT NOT NULL PRIMARY KEY,
			workspace_id TEXT NOT NULL DEFAULT '',
			title TEXT NOT NULL DEFAULT '',
			organization_id TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			is_active BOOLEAN NOT NULL DEFAULT 1
		);
		CREATE TABLE IF NOT EXISTS applications (
			id TEXT NOT NULL PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			client_id TEXT NOT NULL,
			title TEXT NOT NULL DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			is_active BOOLEAN NOT NULL DEFAULT 1
		);
	`); err != nil {
		_ = seedDB.Close()
		t.Fatalf("create org schema: %v", err)
	}
	_ = seedDB.Close()

	reg := dbregistry.New(baseDir, migrationsDir)
	mgr, err := reg.For(orgID)
	if err != nil {
		t.Fatalf("reg.For: %v", err)
	}
	return reg, mgr.Connector.DB
}

func insertOrg(t *testing.T, m *db.DatabaseManager, uuid, slug string) {
	t.Helper()
	if _, err := m.Connector.DB.Exec(
		`INSERT INTO organization (id, organization_id, title) VALUES (?, ?, ?)`,
		uuid, slug, "Org",
	); err != nil {
		t.Fatalf("insert org: %v", err)
	}
}

func insertWorkspaceDB(t *testing.T, orgDB *sql.DB, uuid, slug, orgUUID string) {
	t.Helper()
	if _, err := orgDB.Exec(
		`INSERT INTO workspace (id, workspace_id, title, organization_id) VALUES (?, ?, ?, ?)`,
		uuid, slug, "WS", orgUUID,
	); err != nil {
		t.Fatalf("insert workspace: %v", err)
	}
}

func insertAppDB(t *testing.T, orgDB *sql.DB, uuid, clientID, wsUUID string) {
	t.Helper()
	if _, err := orgDB.Exec(
		`INSERT INTO applications (id, workspace_id, client_id, title) VALUES (?, ?, ?, ?)`,
		uuid, wsUUID, clientID, "App",
	); err != nil {
		t.Fatalf("insert app: %v", err)
	}
}

func TestResolveRedirectTarget_HappyPath(t *testing.T) {
	m := freshMainDB(t)
	reg, orgDB := freshOrgRegistry(t, "org-uuid")
	insertOrg(t, m, "org-uuid", "acme")
	insertWorkspaceDB(t, orgDB, "ws-uuid", "prod", "org-uuid")
	insertAppDB(t, orgDB, "app-uuid", "web", "ws-uuid")

	got, err := resolveRedirectTarget(m, reg, "app-uuid", "org-uuid")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("want redirect target, got nil")
	}
	if got.OrgSlug != "acme" || got.WorkspaceSlug != "prod" || got.ClientID != "web" {
		t.Errorf("triple mismatch: got (%q,%q,%q), want (acme,prod,web)",
			got.OrgSlug, got.WorkspaceSlug, got.ClientID)
	}
}

func TestResolveRedirectTarget_TenantMismatchIsRejected(t *testing.T) {
	m := freshMainDB(t)
	// app-b lives in org-b; caller expects org-a — must be rejected.
	regA, _ := freshOrgRegistry(t, "org-a")
	regB, orgBDB := freshOrgRegistry(t, "org-b")
	insertOrg(t, m, "org-a", "acme")
	insertOrg(t, m, "org-b", "widgets")
	insertWorkspaceDB(t, orgBDB, "ws-b", "staging", "org-b")
	insertAppDB(t, orgBDB, "app-b", "dash", "ws-b")

	// Build a registry that knows both orgs by reusing regA's baseDir for regB.
	// Simplest: create a combined registry backed by a single baseDir.
	_ = regA
	_ = regB

	// Use regB directly: app-b exists; expectedOrgID = "org-a" must fail.
	combinedBaseDir := t.TempDir()
	migrationsDir := t.TempDir()
	for _, orgID := range []string{"org-a", "org-b"} {
		orgDir := filepath.Join(combinedBaseDir, "organization", orgID)
		if err := os.MkdirAll(orgDir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", orgID, err)
		}
	}
	orgBPath := filepath.Join(combinedBaseDir, "organization", "org-b", "users.db")
	sdb, _ := sql.Open("sqlite3", orgBPath)
	_, _ = sdb.Exec(`
		CREATE TABLE IF NOT EXISTS workspace (id TEXT PRIMARY KEY, workspace_id TEXT, title TEXT, organization_id TEXT, created_at DATETIME, is_active BOOLEAN);
		CREATE TABLE IF NOT EXISTS applications (id TEXT PRIMARY KEY, workspace_id TEXT, client_id TEXT, title TEXT, created_at DATETIME, is_active BOOLEAN);
	`)
	_, _ = sdb.Exec(`INSERT INTO workspace (id, workspace_id, title, organization_id) VALUES ('ws-b','staging','WS','org-b')`)
	_, _ = sdb.Exec(`INSERT INTO applications (id, workspace_id, client_id, title) VALUES ('app-b','ws-b','dash','App')`)
	_ = sdb.Close()
	orgAPath := filepath.Join(combinedBaseDir, "organization", "org-a", "users.db")
	sdbA, _ := sql.Open("sqlite3", orgAPath)
	_, _ = sdbA.Exec(`
		CREATE TABLE IF NOT EXISTS workspace (id TEXT PRIMARY KEY, workspace_id TEXT, title TEXT, organization_id TEXT, created_at DATETIME, is_active BOOLEAN);
		CREATE TABLE IF NOT EXISTS applications (id TEXT PRIMARY KEY, workspace_id TEXT, client_id TEXT, title TEXT, created_at DATETIME, is_active BOOLEAN);
	`)
	_ = sdbA.Close()

	reg := dbregistry.New(combinedBaseDir, migrationsDir)
	_, _ = reg.For("org-a")
	_, _ = reg.For("org-b")

	got, err := resolveRedirectTarget(m, reg, "app-b", "org-a")
	if err == nil {
		t.Fatal("expected tenant-mismatch error, got nil")
	}
	if got != nil {
		t.Errorf("want nil target on mismatch, got %+v", got)
	}
}

func TestResolveRedirectTarget_UnknownAppIDIsRejected(t *testing.T) {
	m := freshMainDB(t)
	reg, _ := freshOrgRegistry(t, "org-uuid")
	insertOrg(t, m, "org-uuid", "acme")
	// No applications seeded.

	got, err := resolveRedirectTarget(m, reg, "missing-app-id", "org-uuid")
	if err == nil {
		t.Fatal("expected error for missing app")
	}
	if got != nil {
		t.Errorf("want nil target, got %+v", got)
	}
}

func TestResolveRedirectTarget_EmptyExpectedOrgSkipsTenantCheck(t *testing.T) {
	m := freshMainDB(t)
	reg, orgDB := freshOrgRegistry(t, "org-uuid")
	insertOrg(t, m, "org-uuid", "acme")
	insertWorkspaceDB(t, orgDB, "ws-uuid", "prod", "org-uuid")
	insertAppDB(t, orgDB, "app-uuid", "web", "ws-uuid")

	got, err := resolveRedirectTarget(m, reg, "app-uuid", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil || got.ClientID != "web" {
		t.Errorf("want client_id=web, got %+v", got)
	}
}

func TestResolveRedirectTarget_BrokenChainReturnsError(t *testing.T) {
	m := freshMainDB(t)
	reg, orgDB := freshOrgRegistry(t, "org-uuid")
	insertOrg(t, m, "org-uuid", "acme")
	insertWorkspaceDB(t, orgDB, "ws-uuid", "prod", "org-uuid")
	// App points to a workspace id that doesn't exist in the DB.
	insertAppDB(t, orgDB, "app-uuid", "web", "orphan-workspace")

	got, err := resolveRedirectTarget(m, reg, "app-uuid", "org-uuid")
	if err == nil {
		t.Fatal("expected error for broken chain")
	}
	if got != nil {
		t.Errorf("want nil, got %+v", got)
	}
}
