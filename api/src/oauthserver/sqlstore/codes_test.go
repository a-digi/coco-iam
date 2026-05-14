package sqlstore

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/a-digi/coco-iam/src/oauthserver"
	"github.com/a-digi/coco-iam/src/oauthserver/entity"

	_ "github.com/mattn/go-sqlite3"
)

func openCodesDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec(`
		CREATE TABLE oauth_authorization_codes (
		    code                   TEXT PRIMARY KEY,
		    client_row_id          TEXT NOT NULL,
		    application_id         TEXT NOT NULL,
		    user_id                TEXT NOT NULL,
		    redirect_uri           TEXT NOT NULL,
		    scopes                 TEXT NOT NULL DEFAULT '[]',
		    code_challenge         TEXT NOT NULL DEFAULT '',
		    code_challenge_method  TEXT NOT NULL DEFAULT 'S256',
		    nonce                  TEXT NOT NULL DEFAULT '',
		    created_at             DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
	`); err != nil {
		t.Fatalf("schema: %v", err)
	}
	return db
}

func sampleMint() oauthserver.CodeMintInput {
	return oauthserver.CodeMintInput{
		ClientRowID:         "client-row-1",
		ApplicationID:       "app-1",
		UserID:              "user-1",
		RedirectURI:         "https://app.example/cb",
		Scopes:              []string{"openid", "profile"},
		CodeChallenge:       "challenge-xyz",
		CodeChallengeMethod: "S256",
		Nonce:               "nonce-abc",
	}
}

func TestCodeRepo_MintReturnsOpaqueValue(t *testing.T) {
	r := NewCodeRepo(openCodesDB(t))
	code, err := r.Mint(context.Background(), sampleMint(), 0)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	if code == "" {
		t.Fatal("Mint must return a non-empty code")
	}
	if len(code) < 32 {
		t.Errorf("code looks too short to be opaque-random: %q", code)
	}
}

func TestCodeRepo_ConsumeOnceRoundTrip(t *testing.T) {
	r := NewCodeRepo(openCodesDB(t))
	in := sampleMint()
	code, _ := r.Mint(context.Background(), in, 0)

	got, err := r.ConsumeOnce(context.Background(), code)
	if err != nil {
		t.Fatalf("ConsumeOnce: %v", err)
	}
	if got.UserID != in.UserID {
		t.Errorf("user_id round-trip: %q vs %q", got.UserID, in.UserID)
	}
	if !reflect.DeepEqual(got.Scopes, in.Scopes) {
		t.Errorf("scopes round-trip: %v vs %v", got.Scopes, in.Scopes)
	}
	if got.CodeChallenge != in.CodeChallenge {
		t.Errorf("challenge round-trip mismatch")
	}
	if got.Nonce != in.Nonce {
		t.Errorf("nonce round-trip mismatch")
	}
}

func TestCodeRepo_ConsumeOnceIsActuallyOnce(t *testing.T) {
	r := NewCodeRepo(openCodesDB(t))
	code, _ := r.Mint(context.Background(), sampleMint(), 0)

	if _, err := r.ConsumeOnce(context.Background(), code); err != nil {
		t.Fatalf("first consume: %v", err)
	}
	// Second use must fail with ErrCodeNotFound — replay
	// detection at the persistence layer.
	if _, err := r.ConsumeOnce(context.Background(), code); !errors.Is(err, entity.ErrCodeNotFound) {
		t.Errorf("second consume should be ErrCodeNotFound, got %v", err)
	}
}

func TestCodeRepo_ConsumeUnknownReturnsSentinel(t *testing.T) {
	r := NewCodeRepo(openCodesDB(t))
	_, err := r.ConsumeOnce(context.Background(), "no-such-code")
	if !errors.Is(err, entity.ErrCodeNotFound) {
		t.Errorf("want ErrCodeNotFound, got %v", err)
	}
}

func TestCodeRepo_DeleteExpired(t *testing.T) {
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	r := NewCodeRepo(openCodesDB(t))
	r.Now = func() time.Time { return now }

	// Two old codes + one fresh one.
	r.Mint(context.Background(), sampleMint(), 0)
	r.Mint(context.Background(), sampleMint(), 0)
	now = now.Add(10 * time.Minute)
	r.Mint(context.Background(), sampleMint(), 0)

	removed, err := r.DeleteExpired(context.Background(), now.Add(-5*time.Minute))
	if err != nil {
		t.Fatalf("DeleteExpired: %v", err)
	}
	if removed != 2 {
		t.Errorf("want 2 removed, got %d", removed)
	}
}

func TestCodeRepo_MintDefaultsBlankChallengeMethod(t *testing.T) {
	r := NewCodeRepo(openCodesDB(t))
	in := sampleMint()
	in.CodeChallengeMethod = ""
	code, _ := r.Mint(context.Background(), in, 0)
	got, _ := r.ConsumeOnce(context.Background(), code)
	if got.CodeChallengeMethod != "S256" {
		t.Errorf("blank method should default to S256, got %q", got.CodeChallengeMethod)
	}
}
