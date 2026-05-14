package activation

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const orgActivationsSchema = `
	CREATE TABLE user_activations (
		id TEXT NOT NULL PRIMARY KEY,
		user_id TEXT NOT NULL,
		token_hash TEXT NOT NULL,
		temp_password_hash TEXT NOT NULL,
		expires_at DATETIME NOT NULL,
		consumed_at DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		redirect_organization_slug TEXT,
		redirect_workspace_slug TEXT,
		redirect_application_client_id TEXT
	);
	CREATE UNIQUE INDEX user_activations_token_hash_uindex ON user_activations (token_hash);
`

// freshOrgDB creates an in-memory SQLite database with the per-org
// user_activations schema and returns the raw *sql.DB.
func freshOrgDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(orgActivationsSchema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return db
}

// freshOrgStoreWithDBs builds an OrgStore backed by a map of in-memory DBs
// so tests avoid filesystem I/O and real registry wiring.
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

func TestOrgStoreInsert_RoundTripsRedirectSlugs(t *testing.T) {
	db := freshOrgDB(t)
	s := freshOrgStoreWithDBs(map[string]*sql.DB{"org-1": db})
	expires := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Second)

	row := Row{
		UserID:                "user-1",
		UserType:              UserTypeUser,
		TokenHash:             "hash-1",
		TempPasswordHash:      "bcrypt-temp",
		ExpiresAt:             expires,
		RedirectOrgSlug:       "acme",
		RedirectWorkspaceSlug: "prod",
		RedirectClientID:      "web",
	}
	if err := s.Insert(row, db); err != nil {
		t.Fatalf("insert: %v", err)
	}

	got, gotDB, err := s.FindByTokenHash("hash-1")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if gotDB != db {
		t.Error("FindByTokenHash returned wrong DB pointer")
	}
	if got.RedirectOrgSlug != "acme" || got.RedirectWorkspaceSlug != "prod" || got.RedirectClientID != "web" {
		t.Errorf("slugs mismatch: got (%q,%q,%q), want (acme,prod,web)",
			got.RedirectOrgSlug, got.RedirectWorkspaceSlug, got.RedirectClientID)
	}
	if got.UserType != UserTypeUser {
		t.Errorf("user_type: got %q, want user", got.UserType)
	}
}

func TestOrgStoreInsert_EmptyRedirectSlugsStoreAsNull(t *testing.T) {
	db := freshOrgDB(t)
	s := freshOrgStoreWithDBs(map[string]*sql.DB{"org-1": db})

	row := Row{
		UserID:           "user-2",
		TokenHash:        "hash-2",
		TempPasswordHash: "bcrypt-temp",
		ExpiresAt:        time.Now().Add(time.Hour),
	}
	if err := s.Insert(row, db); err != nil {
		t.Fatalf("insert: %v", err)
	}

	var org, ws, clientID sql.NullString
	err := db.QueryRow(
		`SELECT redirect_organization_slug, redirect_workspace_slug, redirect_application_client_id
		 FROM user_activations WHERE token_hash = ?`, "hash-2",
	).Scan(&org, &ws, &clientID)
	if err != nil {
		t.Fatalf("raw read: %v", err)
	}
	if org.Valid || ws.Valid || clientID.Valid {
		t.Errorf("expected NULLs, got (%v,%v,%v)", org, ws, clientID)
	}
}

func TestOrgStoreLatestPendingForUser_CarriesRedirectSlugs(t *testing.T) {
	db := freshOrgDB(t)
	s := freshOrgStoreWithDBs(map[string]*sql.DB{"org-1": db})

	_ = s.Insert(Row{
		UserID:                "user-3",
		TokenHash:             "hash-3",
		TempPasswordHash:      "bcrypt-temp",
		ExpiresAt:             time.Now().Add(time.Hour),
		RedirectOrgSlug:       "org",
		RedirectWorkspaceSlug: "ws",
		RedirectClientID:      "app",
	}, db)

	latest, err := s.LatestPendingForUser("user-3", db)
	if err != nil {
		t.Fatalf("latest: %v", err)
	}
	if latest == nil {
		t.Fatal("expected a pending row")
	}
	if latest.RedirectOrgSlug != "org" || latest.RedirectWorkspaceSlug != "ws" || latest.RedirectClientID != "app" {
		t.Errorf("slugs: got (%q,%q,%q), want (org,ws,app)",
			latest.RedirectOrgSlug, latest.RedirectWorkspaceSlug, latest.RedirectClientID)
	}
}

func TestOrgStoreConsumeByID_DeletesSiblings(t *testing.T) {
	db := freshOrgDB(t)
	s := freshOrgStoreWithDBs(map[string]*sql.DB{"org-1": db})
	expires := time.Now().Add(time.Hour)

	_ = s.Insert(Row{ID: "act-1", UserID: "user-4", TokenHash: "h1", TempPasswordHash: "x", ExpiresAt: expires}, db)
	_ = s.Insert(Row{ID: "act-2", UserID: "user-4", TokenHash: "h2", TempPasswordHash: "x", ExpiresAt: expires}, db)

	if err := s.ConsumeByID("act-1", db); err != nil {
		t.Fatalf("consume: %v", err)
	}

	var consumed sql.NullString
	if err := db.QueryRow(`SELECT consumed_at FROM user_activations WHERE id = ?`, "act-1").Scan(&consumed); err != nil {
		t.Fatalf("act-1 lookup: %v", err)
	}
	if !consumed.Valid {
		t.Error("act-1 should have consumed_at set")
	}

	var remaining int
	if err := db.QueryRow(`SELECT COUNT(*) FROM user_activations WHERE id = ?`, "act-2").Scan(&remaining); err != nil {
		t.Fatalf("act-2 count: %v", err)
	}
	if remaining != 0 {
		t.Errorf("act-2 should be deleted, got %d rows", remaining)
	}
}

func TestOrgStoreFindByTokenHash_ScansMultipleOrgs(t *testing.T) {
	db1 := freshOrgDB(t)
	db2 := freshOrgDB(t)
	s := freshOrgStoreWithDBs(map[string]*sql.DB{"org-a": db1, "org-b": db2})

	// Insert a row only into org-b.
	_ = s.Insert(Row{
		UserID:           "user-5",
		TokenHash:        "hash-5",
		TempPasswordHash: "bcrypt-temp",
		ExpiresAt:        time.Now().Add(time.Hour),
	}, db2)

	got, gotDB, err := s.FindByTokenHash("hash-5")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if gotDB != db2 {
		t.Error("expected db2 to be returned")
	}
	if got.UserID != "user-5" {
		t.Errorf("user_id: got %q, want user-5", got.UserID)
	}
}

func TestOrgStoreFindByTokenHash_ReturnsErrNotFoundWhenMissing(t *testing.T) {
	db := freshOrgDB(t)
	s := freshOrgStoreWithDBs(map[string]*sql.DB{"org-1": db})

	_, _, err := s.FindByTokenHash("nonexistent-hash")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestOrgStoreLatestPendingForUser_ReturnsNilWhenNone(t *testing.T) {
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
