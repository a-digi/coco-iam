package identities

import (
	"database/sql"
	"errors"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	_, err = db.Exec(`
		CREATE TABLE user_oauth_identities (
		    id                      TEXT NOT NULL PRIMARY KEY,
		    user_id                 TEXT NOT NULL,
		    provider                TEXT NOT NULL,
		    provider_sub            TEXT NOT NULL,
		    email_at_link           TEXT NOT NULL DEFAULT '',
		    email_verified_at_link  INTEGER NOT NULL DEFAULT 0,
		    created_at              DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE UNIQUE INDEX user_oauth_identities_provider_sub_idx
		    ON user_oauth_identities(provider, provider_sub);
		CREATE INDEX user_oauth_identities_user_id_idx
		    ON user_oauth_identities(user_id);
	`)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	return db
}

func TestLink_RoundTrip(t *testing.T) {
	r := New(openTestDB(t))
	id, err := r.Link(LinkInput{
		UserID:              "user-1",
		Provider:            "google",
		ProviderSub:         "google|123",
		EmailAtLink:         "alice@example.com",
		EmailVerifiedAtLink: true,
	})
	if err != nil {
		t.Fatalf("Link: %v", err)
	}
	if id.ID == "" {
		t.Error("Link must mint an id")
	}
	if id.EmailVerifiedAtLink != true {
		t.Error("verified flag lost")
	}
}

func TestFindByProviderSub_Hit(t *testing.T) {
	r := New(openTestDB(t))
	r.Link(LinkInput{
		UserID: "user-1", Provider: "google", ProviderSub: "sub-1",
		EmailAtLink: "alice@example.com", EmailVerifiedAtLink: true,
	})
	got, err := r.FindByProviderSub("google", "sub-1")
	if err != nil {
		t.Fatalf("FindByProviderSub: %v", err)
	}
	if got.UserID != "user-1" {
		t.Errorf("user_id mismatch: %q", got.UserID)
	}
}

func TestFindByProviderSub_MissReturnsSentinel(t *testing.T) {
	r := New(openTestDB(t))
	_, err := r.FindByProviderSub("google", "unknown")
	if !errors.Is(err, ErrIdentityNotFound) {
		t.Errorf("want ErrIdentityNotFound, got %v", err)
	}
}

func TestListForUser_ReturnsAllLinkedProviders(t *testing.T) {
	r := New(openTestDB(t))
	r.Link(LinkInput{UserID: "user-1", Provider: "google", ProviderSub: "g1"})
	r.Link(LinkInput{UserID: "user-1", Provider: "github", ProviderSub: "h1"})
	r.Link(LinkInput{UserID: "user-2", Provider: "google", ProviderSub: "g2"})

	got, err := r.ListForUser("user-1")
	if err != nil {
		t.Fatalf("ListForUser: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 rows, got %d", len(got))
	}
}

func TestLink_DuplicateProviderSubRejected(t *testing.T) {
	r := New(openTestDB(t))
	in := LinkInput{UserID: "user-1", Provider: "google", ProviderSub: "sub-1"}
	if _, err := r.Link(in); err != nil {
		t.Fatalf("first link: %v", err)
	}
	// Second link with the same (provider, sub) — unique index
	// catches it; the caller should preflight via
	// FindByProviderSub, but the DB must enforce regardless.
	if _, err := r.Link(in); err == nil {
		t.Error("duplicate link should have failed")
	}
}

func TestUnlink_RemovesRow(t *testing.T) {
	r := New(openTestDB(t))
	r.Link(LinkInput{UserID: "user-1", Provider: "google", ProviderSub: "g1"})
	if err := r.Unlink("user-1", "google"); err != nil {
		t.Fatalf("Unlink: %v", err)
	}
	if _, err := r.FindByProviderSub("google", "g1"); !errors.Is(err, ErrIdentityNotFound) {
		t.Errorf("row still present after Unlink: %v", err)
	}
}

func TestUnlink_IsIdempotent(t *testing.T) {
	r := New(openTestDB(t))
	if err := r.Unlink("user-1", "google"); err != nil {
		t.Errorf("Unlink on absent row should be no-op, got %v", err)
	}
}
