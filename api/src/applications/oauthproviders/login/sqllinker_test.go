package login

import (
	"database/sql"
	"testing"

	"github.com/a-digi/coco-iam/src/applications/oauthproviders/entity"

	_ "github.com/mattn/go-sqlite3"
)

// newInMemoryUsersDB returns a DB with the per-org users +
// user_oauth_identities schema applied. Tests use a shared
// resolver that returns this DB for any orgID so the linker
// thinks it's talking to the registry.
func newInMemoryUsersDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	_, err = db.Exec(`
		CREATE TABLE users (
		    id                       TEXT PRIMARY KEY,
		    username                 TEXT,
		    email                    TEXT,
		    is_active                INTEGER DEFAULT 1,
		    must_change_password     INTEGER DEFAULT 0,
		    created_at               DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE user_oauth_identities (
		    id                       TEXT PRIMARY KEY,
		    user_id                  TEXT NOT NULL,
		    provider                 TEXT NOT NULL,
		    provider_sub             TEXT NOT NULL,
		    email_at_link            TEXT NOT NULL DEFAULT '',
		    email_verified_at_link   INTEGER NOT NULL DEFAULT 0,
		    created_at               DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE UNIQUE INDEX user_oauth_identities_provider_sub_idx
		    ON user_oauth_identities(provider, provider_sub);
	`)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	return db
}

// newSQLLinker builds a linker whose DB resolver always returns
// the same in-memory DB, skipping the real registry entirely.
func newSQLLinker(t *testing.T) *SQLLinker {
	t.Helper()
	db := newInMemoryUsersDB(t)
	return &SQLLinker{Resolve: func(_ string) (*sql.DB, error) { return db, nil }}
}

func TestSQLLinker_FindByIdentity_Miss(t *testing.T) {
	_ = "org-1"
	lk := newSQLLinker(t)
	_, ok, err := lk.FindByIdentity("org-1", entity.ProviderGoogle, "unknown-sub")
	if err != nil {
		t.Fatalf("FindByIdentity: %v", err)
	}
	if ok {
		t.Error("should miss")
	}
}

func TestSQLLinker_CreateUserFromIdentity_InsertsBothRows(t *testing.T) {
	_ = "org-1"
	lk := newSQLLinker(t)
	id := entity.Identity{
		Provider:      entity.ProviderGoogle,
		Sub:           "google-sub-1",
		Email:         "Alice@Example.com",
		EmailVerified: true,
		FirstName:     "Alice",
		LastName:      "Liddell",
	}
	userID, err := lk.CreateUserFromIdentity("org-1", id)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if userID == "" {
		t.Fatal("empty user id")
	}

	// The identity row should exist for subsequent lookups.
	gotID, ok, err := lk.FindByIdentity("org-1", entity.ProviderGoogle, "google-sub-1")
	if err != nil || !ok {
		t.Fatalf("identity not linked: ok=%v err=%v", ok, err)
	}
	if gotID != userID {
		t.Errorf("uid mismatch: %q vs %q", gotID, userID)
	}

	// Email should be normalised to lower-case.
	emailID, ok, err := lk.FindByEmail("org-1", "alice@example.com")
	if err != nil || !ok || emailID != userID {
		t.Errorf("email lookup broken: ok=%v err=%v id=%q", ok, err, emailID)
	}
}

func TestSQLLinker_FindByEmail_CaseInsensitive(t *testing.T) {
	_ = "org-1"
	lk := newSQLLinker(t)
	_, err := lk.CreateUserFromIdentity("org-1", entity.Identity{
		Provider: entity.ProviderGoogle, Sub: "x",
		Email: "alice@example.com", EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	_, ok, err := lk.FindByEmail("org-1", "ALICE@EXAMPLE.COM")
	if err != nil || !ok {
		t.Errorf("case-insensitive lookup broken: ok=%v err=%v", ok, err)
	}
}

func TestSQLLinker_LinkIdentity_IdempotentOnSameUser(t *testing.T) {
	_ = "org-1"
	lk := newSQLLinker(t)
	uid, _ := lk.CreateUserFromIdentity("org-1", entity.Identity{
		Provider: entity.ProviderGoogle, Sub: "sub-a", Email: "a@b",
	})
	// Linking the same (provider, sub) again to the same user
	// must be a no-op — NOT an error.
	err := lk.LinkIdentity("org-1", uid, entity.Identity{
		Provider: entity.ProviderGoogle, Sub: "sub-a", Email: "a@b",
	})
	if err != nil {
		t.Errorf("idempotent link should succeed, got %v", err)
	}
}

func TestSQLLinker_LinkIdentity_AttachesSecondProvider(t *testing.T) {
	_ = "org-1"
	lk := newSQLLinker(t)
	uid, _ := lk.CreateUserFromIdentity("org-1", entity.Identity{
		Provider: entity.ProviderGoogle, Sub: "g-1",
	})
	err := lk.LinkIdentity("org-1", uid, entity.Identity{
		Provider: entity.ProviderGitHub, Sub: "gh-1",
	})
	if err != nil {
		t.Fatalf("Link: %v", err)
	}
	gotUID, ok, err := lk.FindByIdentity("org-1", entity.ProviderGitHub, "gh-1")
	if err != nil || !ok || gotUID != uid {
		t.Errorf("github identity not attached: %q %v %v", gotUID, ok, err)
	}
}

func TestPickUsername(t *testing.T) {
	cases := []struct {
		name string
		id   entity.Identity
		want string
	}{
		{"first+last", entity.Identity{FirstName: "Alice", LastName: "Liddell"}, "alice.liddell"},
		{"first only", entity.Identity{FirstName: "Alice"}, "alice"},
		{"email local", entity.Identity{Email: "Bob@Example.com"}, "bob"},
		{"fallback", entity.Identity{Provider: entity.ProviderGitHub, Sub: "abc"}, "github:abc"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := pickUsername(tc.id); got != tc.want {
				t.Errorf("got %q want %q", got, tc.want)
			}
		})
	}
}
