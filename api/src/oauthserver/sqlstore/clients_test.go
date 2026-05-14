package sqlstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"

	"github.com/a-digi/coco-iam/src/oauthserver/entity"

	_ "github.com/mattn/go-sqlite3"
)

// plainHasher is a test-only SecretHasher that round-trips the
// plaintext so we can assert on Verify semantics without paying
// bcrypt's cost on every test case.
type plainHasher struct{}

func (plainHasher) Hash(plain string) (string, error) { return "plain:" + plain, nil }
func (plainHasher) Verify(hashed, plain string) error {
	if hashed == "plain:"+plain {
		return nil
	}
	return errors.New("plainHasher: mismatch")
}

func openClientsDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec(`
		CREATE TABLE application_oauth_clients (
		    id                   TEXT NOT NULL PRIMARY KEY,
		    application_id       TEXT NOT NULL,
		    client_id            TEXT NOT NULL,
		    client_secret_hash   TEXT,
		    client_type          TEXT NOT NULL DEFAULT 'confidential',
		    display_name         TEXT NOT NULL DEFAULT '',
		    redirect_uris        TEXT NOT NULL DEFAULT '[]',
		    allowed_scopes       TEXT NOT NULL DEFAULT '[]',
		    require_consent      INTEGER NOT NULL DEFAULT 1,
		    access_token_ttl     INTEGER NOT NULL DEFAULT 3600,
		    refresh_token_ttl    INTEGER NOT NULL DEFAULT 1209600,
		    is_active            INTEGER NOT NULL DEFAULT 1,
		    created_at           DATETIME DEFAULT CURRENT_TIMESTAMP,
		    updated_at           DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE UNIQUE INDEX application_oauth_clients_app_client_id_idx
		    ON application_oauth_clients (application_id, client_id);
	`); err != nil {
		t.Fatalf("schema: %v", err)
	}
	return db
}

func newRepo(t *testing.T) *ClientRepo {
	t.Helper()
	return NewClientRepo(openClientsDB(t), plainHasher{})
}

func sampleConfidential(app, clientID string) InsertInput {
	return InsertInput{
		ApplicationID:   app,
		ClientID:        clientID,
		ClientSecret:    "top-secret",
		Type:            entity.ClientTypeConfidential,
		DisplayName:     "Reporter",
		RedirectURIs:    []string{"https://app.example/cb"},
		AllowedScopes:   []string{"openid", "profile", "email"},
		RequireConsent:  true,
		AccessTokenTTL:  3600,
		RefreshTokenTTL: 1209600,
	}
}

// ---------- unit tests ---------------------------------------------

func TestInsert_RoundTripsAndHashesSecret(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	got, err := r.Insert(ctx, "row-1", sampleConfidential("app-1", "client-a"))
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if got.ID != "row-1" || got.ClientID != "client-a" {
		t.Fatalf("ids wrong: %+v", got)
	}
	if got.SecretHash != "plain:top-secret" {
		t.Fatalf("secret not hashed before storage: %q", got.SecretHash)
	}
	if len(got.RedirectURIs) != 1 || got.RedirectURIs[0] != "https://app.example/cb" {
		t.Fatalf("uris didn't round-trip: %v", got.RedirectURIs)
	}
	if !got.IsActive {
		t.Error("new clients should default to active")
	}
}

func TestInsert_ConfidentialRejectsEmptySecret(t *testing.T) {
	r := newRepo(t)
	in := sampleConfidential("app-1", "client-a")
	in.ClientSecret = ""
	_, err := r.Insert(context.Background(), "id", in)
	if err == nil {
		t.Fatal("empty secret on confidential client must fail")
	}
	var oe *entity.OAuthError
	if !errors.As(err, &oe) || oe.Code != entity.ErrCodeInvalidRequest {
		t.Errorf("want OAuthError invalid_request, got %v", err)
	}
}

func TestInsert_PublicClientWithoutSecret(t *testing.T) {
	r := newRepo(t)
	in := sampleConfidential("app-1", "client-pub")
	in.Type = entity.ClientTypePublic
	in.ClientSecret = ""
	got, err := r.Insert(context.Background(), "pub-1", in)
	if err != nil {
		t.Fatalf("public client insert: %v", err)
	}
	if got.SecretHash != "" {
		t.Errorf("public client should not store a secret, got %q", got.SecretHash)
	}
}

func TestInsert_DuplicateClientIDReportsSentinel(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	if _, err := r.Insert(ctx, "row-1", sampleConfidential("app-1", "dup")); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	_, err := r.Insert(ctx, "row-2", sampleConfidential("app-1", "dup"))
	if !errors.Is(err, entity.ErrDuplicateClient) {
		t.Fatalf("want ErrDuplicateClient, got %v", err)
	}
}

func TestInsert_SameClientIDOnDifferentApp(t *testing.T) {
	// (app_id, client_id) is the unique key — the same
	// client_id is allowed under two different applications.
	r := newRepo(t)
	ctx := context.Background()
	if _, err := r.Insert(ctx, "r1", sampleConfidential("app-A", "shared")); err != nil {
		t.Fatalf("first: %v", err)
	}
	if _, err := r.Insert(ctx, "r2", sampleConfidential("app-B", "shared")); err != nil {
		t.Fatalf("second: %v", err)
	}
}

func TestFindByClientID_Hit(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	r.Insert(ctx, "row-1", sampleConfidential("app-1", "client-a"))
	got, err := r.FindByClientID(ctx, "app-1", "client-a")
	if err != nil {
		t.Fatalf("FindByClientID: %v", err)
	}
	if got.ID != "row-1" {
		t.Errorf("wrong row: %+v", got)
	}
}

func TestFindByClientID_MissReturnsSentinel(t *testing.T) {
	r := newRepo(t)
	_, err := r.FindByClientID(context.Background(), "app-1", "nope")
	if !errors.Is(err, entity.ErrClientNotFound) {
		t.Errorf("want ErrClientNotFound, got %v", err)
	}
}

func TestFindByID_CrossAppIsolation(t *testing.T) {
	// Pin: the primary-key lookup is still scoped by
	// application id so a leaked row id can't cross tenants.
	r := newRepo(t)
	ctx := context.Background()
	r.Insert(ctx, "row-1", sampleConfidential("app-A", "client-a"))
	_, err := r.FindByID(ctx, "app-B", "row-1")
	if !errors.Is(err, entity.ErrClientNotFound) {
		t.Errorf("cross-app read should be blocked, got %v", err)
	}
}

func TestVerifySecret_SuccessAndFailure(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	got, _ := r.Insert(ctx, "row-1", sampleConfidential("app-1", "client-a"))

	if err := r.VerifySecret(ctx, got, "top-secret"); err != nil {
		t.Errorf("correct secret should verify, got %v", err)
	}
	if err := r.VerifySecret(ctx, got, "wrong"); err == nil {
		t.Error("wrong secret should fail")
	}
}

func TestVerifySecret_PublicClientRejectsNonEmpty(t *testing.T) {
	r := newRepo(t)
	in := sampleConfidential("app-1", "client-pub")
	in.Type = entity.ClientTypePublic
	in.ClientSecret = ""
	got, _ := r.Insert(context.Background(), "p-1", in)

	// Empty submission is correct for a public client.
	if err := r.VerifySecret(context.Background(), got, ""); err != nil {
		t.Errorf("public client empty submission should pass, got %v", err)
	}
	// Any submission is wrong.
	err := r.VerifySecret(context.Background(), got, "leaked")
	if err == nil {
		t.Error("public client with submitted secret should be rejected")
	}
}

func TestUpdate_KeepsSecretWhenPointerNil(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	existing, _ := r.Insert(ctx, "row-1", sampleConfidential("app-1", "c"))

	up := UpdateInput{
		DisplayName:     "Renamed",
		RedirectURIs:    existing.RedirectURIs,
		AllowedScopes:   existing.AllowedScopes,
		RequireConsent:  existing.RequireConsent,
		AccessTokenTTL:  existing.AccessTokenTTL,
		RefreshTokenTTL: existing.RefreshTokenTTL,
		IsActive:        existing.IsActive,
	}
	got, err := r.Update(ctx, "app-1", existing.ID, up)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got.SecretHash != existing.SecretHash {
		t.Errorf("nil secret pointer should preserve the stored hash")
	}
	if got.DisplayName != "Renamed" {
		t.Errorf("display name did not update")
	}
}

func TestUpdate_RotatesSecretWhenPointerSet(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	existing, _ := r.Insert(ctx, "row-1", sampleConfidential("app-1", "c"))

	newSecret := "rotated-secret"
	up := UpdateInput{
		ClientSecret:    &newSecret,
		DisplayName:     existing.DisplayName,
		RedirectURIs:    existing.RedirectURIs,
		AllowedScopes:   existing.AllowedScopes,
		RequireConsent:  existing.RequireConsent,
		AccessTokenTTL:  existing.AccessTokenTTL,
		RefreshTokenTTL: existing.RefreshTokenTTL,
		IsActive:        existing.IsActive,
	}
	got, err := r.Update(ctx, "app-1", existing.ID, up)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got.SecretHash != "plain:rotated-secret" {
		t.Errorf("secret did not rotate: %q", got.SecretHash)
	}
	// Old secret fails verify now.
	if err := r.VerifySecret(ctx, got, "top-secret"); err == nil {
		t.Error("old secret should no longer verify after rotation")
	}
}

func TestUpdate_RejectsBlankSecret(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	existing, _ := r.Insert(ctx, "row-1", sampleConfidential("app-1", "c"))

	blank := ""
	_, err := r.Update(ctx, "app-1", existing.ID, UpdateInput{ClientSecret: &blank})
	if err == nil {
		t.Error("explicit empty secret should be rejected")
	}
}

func TestUpdate_UnknownReturnsNotFound(t *testing.T) {
	r := newRepo(t)
	_, err := r.Update(context.Background(), "app-1", "missing", UpdateInput{})
	if !errors.Is(err, entity.ErrClientNotFound) {
		t.Errorf("want ErrClientNotFound, got %v", err)
	}
}

func TestDelete_IdempotentAndRemovesRow(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	r.Insert(ctx, "row-1", sampleConfidential("app-1", "c"))

	if err := r.Delete(ctx, "app-1", "row-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := r.FindByID(ctx, "app-1", "row-1"); !errors.Is(err, entity.ErrClientNotFound) {
		t.Errorf("row still present: %v", err)
	}
	// Second delete on the same id is a no-op.
	if err := r.Delete(ctx, "app-1", "row-1"); err != nil {
		t.Errorf("delete should be idempotent: %v", err)
	}
}

func TestListForApp_ScopedByApplication(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		r.Insert(ctx, fmt.Sprintf("row-%d", i), sampleConfidential("app-1", fmt.Sprintf("c-%d", i)))
	}
	r.Insert(ctx, "row-other", sampleConfidential("app-2", "c-other"))

	got, err := r.ListForApp(ctx, "app-1")
	if err != nil {
		t.Fatalf("ListForApp: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("want 3 rows for app-1, got %d", len(got))
	}
}
