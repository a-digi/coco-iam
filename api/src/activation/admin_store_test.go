package activation

import (
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// freshAdminStore creates an in-memory SQLite database with the
// admin_activations schema and returns an AdminStore wired to it.
func freshAdminStore(t *testing.T) *AdminStore {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec(`
		CREATE TABLE admin_activations (
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
		CREATE UNIQUE INDEX admin_activations_token_hash_uindex ON admin_activations (token_hash);
	`)
	if err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return &AdminStore{db: db}
}

func rawAdminRedirectColumns(t *testing.T, s *AdminStore, id string) (org, ws, clientID sql.NullString) {
	t.Helper()
	err := s.db.QueryRow(
		`SELECT redirect_organization_slug, redirect_workspace_slug, redirect_application_client_id
		 FROM admin_activations WHERE id = ?`, id,
	).Scan(&org, &ws, &clientID)
	if err != nil {
		t.Fatalf("raw read: %v", err)
	}
	return
}

func TestAdminStoreInsert_RoundTripsRedirectSlugs(t *testing.T) {
	s := freshAdminStore(t)
	expires := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Second)

	row := Row{
		UserID:                "admin-1",
		UserType:              UserTypeAdmin,
		TokenHash:             "hash-1",
		TempPasswordHash:      "bcrypt-temp",
		ExpiresAt:             expires,
		RedirectOrgSlug:       "acme",
		RedirectWorkspaceSlug: "prod",
		RedirectClientID:      "web",
	}
	if err := s.Insert(row); err != nil {
		t.Fatalf("insert: %v", err)
	}

	got, err := s.FindByTokenHash("hash-1")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got.RedirectOrgSlug != "acme" || got.RedirectWorkspaceSlug != "prod" || got.RedirectClientID != "web" {
		t.Errorf("slugs mismatch: got (%q,%q,%q), want (acme,prod,web)",
			got.RedirectOrgSlug, got.RedirectWorkspaceSlug, got.RedirectClientID)
	}
	if got.UserID != "admin-1" {
		t.Errorf("user_id: got %q, want admin-1", got.UserID)
	}
	if got.UserType != UserTypeAdmin {
		t.Errorf("user_type: got %q, want admin", got.UserType)
	}
}

func TestAdminStoreInsert_EmptyRedirectSlugsStoreAsNull(t *testing.T) {
	s := freshAdminStore(t)
	row := Row{
		UserID:           "admin-2",
		UserType:         UserTypeAdmin,
		TokenHash:        "hash-2",
		TempPasswordHash: "bcrypt-temp",
		ExpiresAt:        time.Now().Add(time.Hour),
	}
	if err := s.Insert(row); err != nil {
		t.Fatalf("insert: %v", err)
	}

	var id string
	if err := s.db.QueryRow(`SELECT id FROM admin_activations WHERE token_hash = ?`, "hash-2").Scan(&id); err != nil {
		t.Fatalf("id lookup: %v", err)
	}

	org, ws, clientID := rawAdminRedirectColumns(t, s, id)
	if org.Valid {
		t.Errorf("org slug stored as %q, want NULL", org.String)
	}
	if ws.Valid {
		t.Errorf("ws slug stored as %q, want NULL", ws.String)
	}
	if clientID.Valid {
		t.Errorf("client id stored as %q, want NULL", clientID.String)
	}

	got, err := s.FindByTokenHash("hash-2")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got.RedirectOrgSlug != "" || got.RedirectWorkspaceSlug != "" || got.RedirectClientID != "" {
		t.Errorf("scanRow should read NULLs as empty strings, got (%q,%q,%q)",
			got.RedirectOrgSlug, got.RedirectWorkspaceSlug, got.RedirectClientID)
	}
}

func TestAdminStoreLatestPendingForUser_CarriesRedirectSlugs(t *testing.T) {
	s := freshAdminStore(t)
	if err := s.Insert(Row{
		UserID:                "admin-3",
		UserType:              UserTypeAdmin,
		TokenHash:             "hash-3",
		TempPasswordHash:      "bcrypt-temp",
		ExpiresAt:             time.Now().Add(time.Hour),
		RedirectOrgSlug:       "org",
		RedirectWorkspaceSlug: "ws",
		RedirectClientID:      "app",
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	latest, err := s.LatestPendingForUser("admin-3")
	if err != nil {
		t.Fatalf("latest: %v", err)
	}
	if latest == nil {
		t.Fatal("expected a pending row")
	}
	if latest.RedirectOrgSlug != "org" || latest.RedirectWorkspaceSlug != "ws" || latest.RedirectClientID != "app" {
		t.Errorf("latest slugs: got (%q,%q,%q), want (org,ws,app)",
			latest.RedirectOrgSlug, latest.RedirectWorkspaceSlug, latest.RedirectClientID)
	}
}

func TestAdminStoreConsumeByID_DeletesSiblings(t *testing.T) {
	s := freshAdminStore(t)
	expires := time.Now().Add(time.Hour)

	row1 := Row{ID: "act-1", UserID: "admin-4", UserType: UserTypeAdmin, TokenHash: "h1", TempPasswordHash: "x", ExpiresAt: expires}
	row2 := Row{ID: "act-2", UserID: "admin-4", UserType: UserTypeAdmin, TokenHash: "h2", TempPasswordHash: "x", ExpiresAt: expires}
	if err := s.Insert(row1); err != nil {
		t.Fatalf("insert 1: %v", err)
	}
	if err := s.Insert(row2); err != nil {
		t.Fatalf("insert 2: %v", err)
	}

	if err := s.ConsumeByID("act-1"); err != nil {
		t.Fatalf("consume: %v", err)
	}

	var consumed sql.NullString
	if err := s.db.QueryRow(`SELECT consumed_at FROM admin_activations WHERE id = ?`, "act-1").Scan(&consumed); err != nil {
		t.Fatalf("act-1 lookup: %v", err)
	}
	if !consumed.Valid {
		t.Error("act-1 should have consumed_at set")
	}

	var remaining int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM admin_activations WHERE id = ?`, "act-2").Scan(&remaining); err != nil {
		t.Fatalf("act-2 count: %v", err)
	}
	if remaining != 0 {
		t.Errorf("act-2 should be deleted, got %d rows", remaining)
	}
}

func TestAdminStoreDeletePendingForUser_RemovesUnconsumed(t *testing.T) {
	s := freshAdminStore(t)
	expires := time.Now().Add(time.Hour)

	_ = s.Insert(Row{UserID: "admin-5", TokenHash: "h5a", TempPasswordHash: "x", ExpiresAt: expires})
	_ = s.Insert(Row{UserID: "admin-5", TokenHash: "h5b", TempPasswordHash: "x", ExpiresAt: expires})

	if err := s.DeletePendingForUser("admin-5"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM admin_activations WHERE user_id = ?`, "admin-5").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 rows, got %d", count)
	}
}

func TestAdminStoreLatestPendingForUser_ReturnsNilWhenNone(t *testing.T) {
	s := freshAdminStore(t)
	latest, err := s.LatestPendingForUser("no-such-user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if latest != nil {
		t.Errorf("expected nil, got row %+v", latest)
	}
}
