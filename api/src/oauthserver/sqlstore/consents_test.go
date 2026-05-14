package sqlstore

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"

	"github.com/a-digi/coco-iam/src/oauthserver/entity"

	_ "github.com/mattn/go-sqlite3"
)

func openConsentsDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec(`
		CREATE TABLE oauth_user_consents (
		    id              TEXT PRIMARY KEY,
		    user_id         TEXT NOT NULL,
		    client_row_id   TEXT NOT NULL,
		    granted_scopes  TEXT NOT NULL DEFAULT '[]',
		    granted_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		    revoked_at      DATETIME
		);
		CREATE UNIQUE INDEX oauth_user_consents_user_client_idx
		    ON oauth_user_consents(user_id, client_row_id);
	`); err != nil {
		t.Fatalf("schema: %v", err)
	}
	return db
}

func newConsentRepo(t *testing.T) *ConsentRepo {
	t.Helper()
	db := openConsentsDB(t)
	return NewConsentRepo(func(_ string) (*sql.DB, error) { return db, nil })
}

func TestConsentRepo_RecordAndLoad(t *testing.T) {
	r := newConsentRepo(t)
	if err := r.Record(context.Background(), "org-1", "user-1", "client-1", []string{"openid", "email"}); err != nil {
		t.Fatalf("Record: %v", err)
	}
	got, err := r.Load(context.Background(), "org-1", "user-1", "client-1")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reflect.DeepEqual(got.GrantedScopes, []string{"openid", "email"}) {
		t.Errorf("scopes round-trip: %v", got.GrantedScopes)
	}
	if got.IsRevoked() {
		t.Error("fresh consent reported as revoked")
	}
}

func TestConsentRepo_LoadMissReturnsSentinel(t *testing.T) {
	r := newConsentRepo(t)
	_, err := r.Load(context.Background(), "org-1", "user-x", "client-x")
	if !errors.Is(err, entity.ErrConsentNotFound) {
		t.Errorf("want ErrConsentNotFound, got %v", err)
	}
}

func TestConsentRepo_RecordReplacesScopes(t *testing.T) {
	// Re-recording for the same (user, client) replaces the
	// scope set rather than appending.
	r := newConsentRepo(t)
	r.Record(context.Background(), "org-1", "user-1", "client-1", []string{"openid"})
	r.Record(context.Background(), "org-1", "user-1", "client-1", []string{"openid", "email", "profile"})

	got, err := r.Load(context.Background(), "org-1", "user-1", "client-1")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got.GrantedScopes) != 3 {
		t.Errorf("want widened scope set, got %v", got.GrantedScopes)
	}
}

func TestConsentRepo_RevokeMakesLoadReturnNotFound(t *testing.T) {
	r := newConsentRepo(t)
	r.Record(context.Background(), "org-1", "user-1", "client-1", []string{"openid"})
	if err := r.Revoke(context.Background(), "org-1", "user-1", "client-1"); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	_, err := r.Load(context.Background(), "org-1", "user-1", "client-1")
	if !errors.Is(err, entity.ErrConsentNotFound) {
		t.Errorf("revoked consent must read as missing, got %v", err)
	}
}

func TestConsentRepo_RecordAfterRevokeReinstates(t *testing.T) {
	// Admin revokes, user later re-consents → row must be
	// reusable, not a duplicate-key error.
	r := newConsentRepo(t)
	r.Record(context.Background(), "org-1", "user-1", "client-1", []string{"openid"})
	r.Revoke(context.Background(), "org-1", "user-1", "client-1")
	if err := r.Record(context.Background(), "org-1", "user-1", "client-1", []string{"openid", "email"}); err != nil {
		t.Fatalf("Record after Revoke: %v", err)
	}
	got, err := r.Load(context.Background(), "org-1", "user-1", "client-1")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.IsRevoked() {
		t.Error("re-recorded consent should be active")
	}
	if len(got.GrantedScopes) != 2 {
		t.Errorf("scopes after re-record: %v", got.GrantedScopes)
	}
}

func TestConsentRepo_NilResolverIsError(t *testing.T) {
	r := &ConsentRepo{Resolve: nil}
	_, err := r.Load(context.Background(), "org-1", "u", "c")
	if err == nil {
		t.Error("nil resolver should error")
	}
}
