package repository

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/a-digi/coco-iam/src/applications/oauthproviders/entity"
	_ "github.com/mattn/go-sqlite3"
)

// openTestDB gives every test a private in-memory SQLite with
// the application_oauth_providers schema applied. The schema
// mirrors the migration at
// api/config/db/migrations/30_04_2026_14_00_00.sql.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	_, err = db.Exec(`
		CREATE TABLE application_oauth_providers (
		    id                  TEXT NOT NULL PRIMARY KEY,
		    application_id      TEXT NOT NULL,
		    provider            TEXT NOT NULL,
		    client_id           TEXT NOT NULL,
		    client_secret_enc   TEXT NOT NULL,
		    discovery_url       TEXT NOT NULL DEFAULT '',
		    authorize_url       TEXT NOT NULL DEFAULT '',
		    token_url           TEXT NOT NULL DEFAULT '',
		    userinfo_url        TEXT NOT NULL DEFAULT '',
		    scopes              TEXT NOT NULL DEFAULT '',
		    allow_login         INTEGER NOT NULL DEFAULT 1,
		    allow_registration  INTEGER NOT NULL DEFAULT 0,
		    is_active           INTEGER NOT NULL DEFAULT 1,
		    created_at          DATETIME DEFAULT CURRENT_TIMESTAMP,
		    updated_at          DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE UNIQUE INDEX application_oauth_providers_app_provider_idx
		    ON application_oauth_providers (application_id, provider);
	`)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	return db
}

func sampleInsert(app string, p entity.Provider) InsertInput {
	return InsertInput{
		ApplicationID:     app,
		Provider:          p,
		ClientID:          "cid-" + string(p),
		ClientSecret:      "top-secret-" + string(p),
		DiscoveryURL:      "https://" + string(p) + "/.well-known",
		AuthorizeURL:      "https://" + string(p) + "/auth",
		TokenURL:          "https://" + string(p) + "/token",
		UserinfoURL:       "https://" + string(p) + "/userinfo",
		Scopes:            []string{"openid", "email", "profile"},
		AllowLogin:        true,
		AllowRegistration: true,
	}
}

func TestInsert_RoundTripsAndDecryptsSecret(t *testing.T) {
	r := New(openTestDB(t))
	in := sampleInsert("app-1", entity.ProviderGoogle)
	cfg, err := r.Insert(in)
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if cfg.ID == "" {
		t.Errorf("Insert should mint an id")
	}
	if cfg.ClientSecret != in.ClientSecret {
		t.Errorf("Insert should return the plaintext secret")
	}
	// Read it back — secret must decrypt to the original.
	got, err := r.FindByID("app-1", cfg.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got.ClientSecret != in.ClientSecret {
		t.Errorf("decrypt drift: %q vs %q", got.ClientSecret, in.ClientSecret)
	}
	if len(got.Scopes) != 3 || got.Scopes[0] != "openid" {
		t.Errorf("scopes: %v", got.Scopes)
	}
}

func TestInsert_DuplicateProviderIsReported(t *testing.T) {
	r := New(openTestDB(t))
	if _, err := r.Insert(sampleInsert("app-1", entity.ProviderGoogle)); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	_, err := r.Insert(sampleInsert("app-1", entity.ProviderGoogle))
	if !errors.Is(err, entity.ErrDuplicateProvider) {
		t.Fatalf("want ErrDuplicateProvider, got %v", err)
	}
}

func TestInsert_SameProviderDifferentAppIsAllowed(t *testing.T) {
	// (app, provider) is the unique key. An admin configuring
	// Google on two different apps must succeed.
	r := New(openTestDB(t))
	if _, err := r.Insert(sampleInsert("app-1", entity.ProviderGoogle)); err != nil {
		t.Fatalf("first: %v", err)
	}
	if _, err := r.Insert(sampleInsert("app-2", entity.ProviderGoogle)); err != nil {
		t.Fatalf("second: %v", err)
	}
}

func TestFindByID_NotFoundReturnsSentinel(t *testing.T) {
	r := New(openTestDB(t))
	_, err := r.FindByID("app-1", "missing")
	if !errors.Is(err, entity.ErrProviderNotFound) {
		t.Errorf("want ErrProviderNotFound, got %v", err)
	}
}

func TestFindByID_CrossAppReadsAreBlocked(t *testing.T) {
	// Pin: even a correct id from a different app can't read.
	r := New(openTestDB(t))
	cfg, _ := r.Insert(sampleInsert("app-A", entity.ProviderGoogle))
	_, err := r.FindByID("app-B", cfg.ID)
	if !errors.Is(err, entity.ErrProviderNotFound) {
		t.Errorf("cross-app lookup should fail, got %v", err)
	}
}

func TestFindByProvider_Hit(t *testing.T) {
	r := New(openTestDB(t))
	r.Insert(sampleInsert("app-1", entity.ProviderGoogle))
	got, err := r.FindByProvider("app-1", entity.ProviderGoogle)
	if err != nil {
		t.Fatalf("FindByProvider: %v", err)
	}
	if got.Provider != entity.ProviderGoogle {
		t.Errorf("wrong provider: %v", got.Provider)
	}
}

func TestListForApp_ReturnsOnlyOwnedRows(t *testing.T) {
	r := New(openTestDB(t))
	r.Insert(sampleInsert("app-1", entity.ProviderGoogle))
	r.Insert(sampleInsert("app-1", entity.ProviderGitHub))
	r.Insert(sampleInsert("app-2", entity.ProviderMicrosoft))

	list, err := r.ListForApp("app-1")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("want 2 rows, got %d", len(list))
	}
}

func TestUpdate_ClientSecretNilPreservesStoredSecret(t *testing.T) {
	r := New(openTestDB(t))
	created, _ := r.Insert(sampleInsert("app-1", entity.ProviderGoogle))

	// Update without touching the secret.
	up := UpdateInput{
		ClientID:          "new-cid",
		ClientSecret:      nil,
		DiscoveryURL:      created.DiscoveryURL,
		AuthorizeURL:      created.AuthorizeURL,
		TokenURL:          created.TokenURL,
		UserinfoURL:       created.UserinfoURL,
		Scopes:            created.Scopes,
		AllowLogin:        false,
		AllowRegistration: false,
		IsActive:          true,
	}
	_, err := r.Update("app-1", created.ID, up)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ := r.FindByID("app-1", created.ID)
	if got.ClientSecret != created.ClientSecret {
		t.Errorf("secret should have been preserved: %q vs %q",
			got.ClientSecret, created.ClientSecret)
	}
	if got.ClientID != "new-cid" {
		t.Errorf("client id did not update: %q", got.ClientID)
	}
	if got.AllowLogin || got.AllowRegistration {
		t.Errorf("flags did not update")
	}
}

func TestUpdate_ClientSecretPtrRotatesValue(t *testing.T) {
	r := New(openTestDB(t))
	created, _ := r.Insert(sampleInsert("app-1", entity.ProviderGoogle))

	newSecret := "rotated-secret"
	up := UpdateInput{
		ClientID:          created.ClientID,
		ClientSecret:      &newSecret,
		DiscoveryURL:      created.DiscoveryURL,
		AuthorizeURL:      created.AuthorizeURL,
		TokenURL:          created.TokenURL,
		UserinfoURL:       created.UserinfoURL,
		Scopes:            created.Scopes,
		AllowLogin:        true,
		AllowRegistration: true,
		IsActive:          true,
	}
	if _, err := r.Update("app-1", created.ID, up); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ := r.FindByID("app-1", created.ID)
	if got.ClientSecret != newSecret {
		t.Errorf("secret did not rotate: got %q", got.ClientSecret)
	}
}

func TestUpdate_UnknownRowReturnsNotFound(t *testing.T) {
	r := New(openTestDB(t))
	_, err := r.Update("app-1", "missing", UpdateInput{})
	if !errors.Is(err, entity.ErrProviderNotFound) {
		t.Errorf("want ErrProviderNotFound, got %v", err)
	}
}

func TestDelete_RemovesRowAndIsIdempotent(t *testing.T) {
	r := New(openTestDB(t))
	cfg, _ := r.Insert(sampleInsert("app-1", entity.ProviderGoogle))

	if err := r.Delete("app-1", cfg.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := r.FindByID("app-1", cfg.ID); !errors.Is(err, entity.ErrProviderNotFound) {
		t.Errorf("row still present: %v", err)
	}
	// Second delete on the same id should be a no-op.
	if err := r.Delete("app-1", cfg.ID); err != nil {
		t.Errorf("delete should be idempotent: %v", err)
	}
}

func TestIsAllowedProvider(t *testing.T) {
	for _, p := range []string{"google", "github", "microsoft"} {
		if !entity.IsAllowedProvider(p) {
			t.Errorf("%q should be allowed", p)
		}
	}
	for _, p := range []string{"", "facebook", "apple", "GOOGLE", "custom"} {
		if entity.IsAllowedProvider(p) {
			t.Errorf("%q should not be allowed", p)
		}
	}
}
