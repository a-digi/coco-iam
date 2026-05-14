package repository

import (
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/a-digi/coco-iam/src/applications/apicredentials/entity"
	_ "github.com/mattn/go-sqlite3"
)

// freshRepo opens an in-memory SQLite and applies the
// application_api_credentials schema. Schema kept inline so a drift
// between this test and the migration file surfaces as a test failure.
func freshRepo(t *testing.T) (*Repository, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	schema := `
		CREATE TABLE application_api_credentials (
			id              TEXT NOT NULL PRIMARY KEY,
			application_id  TEXT NOT NULL,
			api_id          TEXT NOT NULL UNIQUE,
			secret_hash     TEXT NOT NULL,
			label           TEXT NOT NULL DEFAULT '',
			purposes        TEXT NOT NULL DEFAULT '[]',
			expires_at      DATETIME NOT NULL,
			is_active       BOOLEAN NOT NULL DEFAULT 1,
			last_used_at    DATETIME,
			created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
			revoked_at      DATETIME
		);
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return New(db), db
}

func sampleCredential(apiID string) entity.Credential {
	return entity.Credential{
		ID:            "cred-" + apiID,
		ApplicationID: "app-1",
		APIID:         apiID,
		SecretHash:    "$2a$10$fakeHashForTest0000000000000000000000000",
		Label:         "test credential",
		ExpiresAt:     time.Now().Add(24 * time.Hour),
		IsActive:      true,
	}
}

func TestInsertAndFindByAPIID_RoundTrip(t *testing.T) {
	repo, _ := freshRepo(t)
	if err := repo.Insert(sampleCredential("api-one"), []string{"security_key:read"}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	got, purposes, err := repo.FindByAPIID("api-one")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got.APIID != "api-one" {
		t.Errorf("api_id: want api-one, got %q", got.APIID)
	}
	if got.ApplicationID != "app-1" {
		t.Errorf("application_id: want app-1, got %q", got.ApplicationID)
	}
	if got.Label != "test credential" {
		t.Errorf("label round-trip failed: %q", got.Label)
	}
	if len(purposes) != 1 || purposes[0] != "security_key:read" {
		t.Errorf("purposes: want [security_key:read], got %v", purposes)
	}
	if !got.IsActive {
		t.Error("is_active should be true after insert")
	}
	if got.RevokedAt != nil {
		t.Error("revoked_at should be nil after insert")
	}
}

func TestFindByAPIID_MissingReturnsErrNotFound(t *testing.T) {
	repo, _ := freshRepo(t)
	_, _, err := repo.FindByAPIID("does-not-exist")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestInsert_PurposesAreJSONEncodedInStorage(t *testing.T) {
	// Pin the on-disk encoding: the DB must hold `["a","b"]`, not
	// a Go-formatted slice. Downstream readers (backup tools, ad-hoc
	// SQL) rely on valid JSON.
	repo, rawDB := freshRepo(t)
	if err := repo.Insert(sampleCredential("api-two"), []string{"a", "b"}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	var raw string
	if err := rawDB.QueryRow(
		`SELECT purposes FROM application_api_credentials WHERE api_id = ?`, "api-two",
	).Scan(&raw); err != nil {
		t.Fatalf("raw read: %v", err)
	}
	if raw != `["a","b"]` {
		t.Errorf("purposes on disk: want %q, got %q", `["a","b"]`, raw)
	}
}

func TestInsert_NilPurposesStoresEmptyJSONArray(t *testing.T) {
	// nil and []string{} should both serialise to `[]` — never NULL
	// and never the Go literal `null`. Handlers rely on purposes
	// always being iterable.
	repo, rawDB := freshRepo(t)
	if err := repo.Insert(sampleCredential("api-three"), nil); err != nil {
		t.Fatalf("insert: %v", err)
	}
	var raw string
	if err := rawDB.QueryRow(
		`SELECT purposes FROM application_api_credentials WHERE api_id = ?`, "api-three",
	).Scan(&raw); err != nil {
		t.Fatalf("raw read: %v", err)
	}
	if raw != `[]` {
		t.Errorf("purposes on disk: want [], got %q", raw)
	}
	_, purposes, err := repo.FindByAPIID("api-three")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if purposes == nil || len(purposes) != 0 {
		t.Errorf("decoded purposes: want empty non-nil slice, got %v", purposes)
	}
}

func TestListForApplication_OrdersByCreatedAtDesc(t *testing.T) {
	repo, _ := freshRepo(t)
	// Insert three credentials for the same app. Because
	// `created_at DEFAULT CURRENT_TIMESTAMP` has only second
	// resolution in SQLite, a tiny delay between inserts is
	// required to guarantee distinct timestamps.
	for _, apiID := range []string{"a", "b", "c"} {
		if err := repo.Insert(sampleCredential(apiID), []string{"p"}); err != nil {
			t.Fatalf("insert %s: %v", apiID, err)
		}
		time.Sleep(1100 * time.Millisecond)
	}
	creds, _, err := repo.ListForApplication("app-1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(creds) != 3 {
		t.Fatalf("want 3 rows, got %d", len(creds))
	}
	if creds[0].APIID != "c" || creds[1].APIID != "b" || creds[2].APIID != "a" {
		t.Errorf("order: want [c,b,a], got [%s,%s,%s]", creds[0].APIID, creds[1].APIID, creds[2].APIID)
	}
}

func TestListForApplication_EmptyWhenOtherAppOnly(t *testing.T) {
	repo, _ := freshRepo(t)
	other := sampleCredential("other")
	other.ApplicationID = "app-2"
	if err := repo.Insert(other, []string{"p"}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	creds, _, err := repo.ListForApplication("app-1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(creds) != 0 {
		t.Errorf("want empty list, got %d rows", len(creds))
	}
}

func TestRevoke_SoftRevokesAndSetsRevokedAt(t *testing.T) {
	repo, _ := freshRepo(t)
	if err := repo.Insert(sampleCredential("api-four"), []string{"p"}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := repo.Revoke("cred-api-four"); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	got, _, err := repo.FindByAPIID("api-four")
	if err != nil {
		t.Fatalf("find after revoke: %v", err)
	}
	if got.IsActive {
		t.Error("is_active should be false after revoke")
	}
	if got.RevokedAt == nil {
		t.Error("revoked_at should be set after revoke")
	}
}

func TestRevoke_OnAlreadyRevokedReturnsErrNotFound(t *testing.T) {
	// Revoke is idempotent-by-error: calling it twice reports
	// ErrNotFound the second time, matching the "row no longer
	// matches the active-filter" semantics. Handlers treat this
	// the same as a missing id.
	repo, _ := freshRepo(t)
	if err := repo.Insert(sampleCredential("api-five"), []string{"p"}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := repo.Revoke("cred-api-five"); err != nil {
		t.Fatalf("first revoke: %v", err)
	}
	err := repo.Revoke("cred-api-five")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("want ErrNotFound on second revoke, got %v", err)
	}
}

func TestTouchLastUsed_StampsTimestamp(t *testing.T) {
	repo, _ := freshRepo(t)
	if err := repo.Insert(sampleCredential("api-six"), []string{"p"}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	// Fresh row: last_used_at is NULL.
	before, _, err := repo.FindByAPIID("api-six")
	if err != nil {
		t.Fatalf("find pre-touch: %v", err)
	}
	if before.LastUsedAt != nil {
		t.Error("last_used_at should start NULL")
	}

	if err := repo.TouchLastUsed("cred-api-six"); err != nil {
		t.Fatalf("touch: %v", err)
	}
	after, _, err := repo.FindByAPIID("api-six")
	if err != nil {
		t.Fatalf("find post-touch: %v", err)
	}
	if after.LastUsedAt == nil {
		t.Error("last_used_at should be set after touch")
	}
}

func TestInsert_DuplicateAPIIDIsRejected(t *testing.T) {
	// Unique index on api_id must block duplicate issuance — two
	// credentials sharing the same public identifier would be a
	// security disaster.
	repo, _ := freshRepo(t)
	if err := repo.Insert(sampleCredential("dup"), []string{"p"}); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	err := repo.Insert(sampleCredential("dup"), []string{"p"})
	if err == nil {
		t.Fatal("want unique-index violation, got nil")
	}
}

func TestFindByAPIID_MalformedPurposesJSONFailsLoud(t *testing.T) {
	// Data-integrity check: if someone hand-edited the DB and put
	// garbage in `purposes`, the scan must error rather than quietly
	// treat it as "no purposes" and let auth pass.
	repo, rawDB := freshRepo(t)
	_, err := rawDB.Exec(
		`INSERT INTO application_api_credentials
		 (id, application_id, api_id, secret_hash, expires_at, purposes)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		"bad-row", "app-1", "api-bad", "h",
		time.Now().Add(time.Hour).Format("2006-01-02 15:04:05"),
		"not json",
	)
	if err != nil {
		t.Fatalf("seed bad row: %v", err)
	}
	_, _, err = repo.FindByAPIID("api-bad")
	if err == nil {
		t.Fatal("want error on malformed purposes JSON, got nil")
	}
}
