package sqlstore

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/a-digi/coco-iam/src/oauthserver/entity"

	_ "github.com/mattn/go-sqlite3"
)

func openRefreshDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec(`
		CREATE TABLE oauth_refresh_tokens (
		    id              TEXT PRIMARY KEY,
		    token_hash      TEXT NOT NULL UNIQUE,
		    client_row_id   TEXT NOT NULL,
		    application_id  TEXT NOT NULL,
		    user_id         TEXT NOT NULL,
		    scopes          TEXT NOT NULL DEFAULT '[]',
		    issued_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		    expires_at      DATETIME NOT NULL,
		    revoked_at      DATETIME,
		    replaced_by_id  TEXT
		);
	`); err != nil {
		t.Fatalf("schema: %v", err)
	}
	return db
}

func TestRefreshRepo_MintAndFindUnconsumed(t *testing.T) {
	r := NewRefreshRepo(openRefreshDB(t))
	raw, rec, err := r.Mint(context.Background(), "client-1", "app-1", "user-1", []string{"openid", "email"}, time.Hour)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	if raw == "" || rec.ID == "" {
		t.Fatal("Mint must return raw + record")
	}
	got, err := r.FindUnconsumed(context.Background(), raw)
	if err != nil {
		t.Fatalf("FindUnconsumed: %v", err)
	}
	if got.UserID != "user-1" || len(got.Scopes) != 2 {
		t.Errorf("unexpected record: %+v", got)
	}
}

func TestRefreshRepo_FindUnconsumed_MissReturnsSentinel(t *testing.T) {
	r := NewRefreshRepo(openRefreshDB(t))
	_, err := r.FindUnconsumed(context.Background(), "no-such-token")
	if !errors.Is(err, entity.ErrRefreshNotFound) {
		t.Errorf("want ErrRefreshNotFound, got %v", err)
	}
}

func TestRefreshRepo_RotateMarksOldConsumedLinkedToNew(t *testing.T) {
	r := NewRefreshRepo(openRefreshDB(t))
	raw1, rec1, _ := r.Mint(context.Background(), "client-1", "app-1", "user-1", []string{"openid"}, time.Hour)
	_, rec2, _ := r.Mint(context.Background(), "client-1", "app-1", "user-1", []string{"openid"}, time.Hour)
	if err := r.Rotate(context.Background(), rec1.ID, rec2.ID); err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	// Old token raw should no longer find as unconsumed AND
	// the record carries replaced_by_id, signalling replay.
	_, err := r.FindUnconsumed(context.Background(), raw1)
	if !errors.Is(err, entity.ErrReplayDetected) {
		t.Errorf("want ErrReplayDetected on old token reuse, got %v", err)
	}
}

func TestRefreshRepo_RevokeMakesTokenInactive(t *testing.T) {
	r := NewRefreshRepo(openRefreshDB(t))
	raw, _, _ := r.Mint(context.Background(), "client-1", "app-1", "user-1", []string{"openid"}, time.Hour)
	if err := r.Revoke(context.Background(), raw); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	_, err := r.FindUnconsumed(context.Background(), raw)
	if !errors.Is(err, entity.ErrRefreshNotFound) {
		t.Errorf("revoked token should be missing, got %v", err)
	}
}

func TestRefreshRepo_RevokeIsIdempotent(t *testing.T) {
	r := NewRefreshRepo(openRefreshDB(t))
	if err := r.Revoke(context.Background(), "no-such-token"); err != nil {
		t.Errorf("revoke on absent should be no-op, got %v", err)
	}
}

func TestRefreshRepo_ExpiredTokenIsMissing(t *testing.T) {
	r := NewRefreshRepo(openRefreshDB(t))
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	r.Now = func() time.Time { return now }

	raw, _, _ := r.Mint(context.Background(), "client-1", "app-1", "user-1", []string{"openid"}, time.Hour)
	now = now.Add(2 * time.Hour) // jump past expiry

	_, err := r.FindUnconsumed(context.Background(), raw)
	if !errors.Is(err, entity.ErrRefreshNotFound) {
		t.Errorf("expired token should be missing, got %v", err)
	}
}

func TestRefreshRepo_RevokeFamilyWalksChain(t *testing.T) {
	// Build a chain: T0 -> T1 -> T2 (each the rotated child
	// of the previous), then RevokeFamily with the middle id.
	// All three must end up revoked.
	r := NewRefreshRepo(openRefreshDB(t))
	ctx := context.Background()
	_, rec0, _ := r.Mint(ctx, "c", "app", "u", []string{"openid"}, time.Hour)
	_, rec1, _ := r.Mint(ctx, "c", "app", "u", []string{"openid"}, time.Hour)
	_, rec2, _ := r.Mint(ctx, "c", "app", "u", []string{"openid"}, time.Hour)
	if err := r.Rotate(ctx, rec0.ID, rec1.ID); err != nil {
		t.Fatalf("rotate 0->1: %v", err)
	}
	if err := r.Rotate(ctx, rec1.ID, rec2.ID); err != nil {
		t.Fatalf("rotate 1->2: %v", err)
	}
	if err := r.RevokeFamily(ctx, rec1.ID); err != nil {
		t.Fatalf("RevokeFamily: %v", err)
	}

	// Each row's revoked_at should now be set.
	for _, id := range []string{rec0.ID, rec1.ID, rec2.ID} {
		var revoked sql.NullString
		if err := openRefreshAssertRevoked(t, r, id, &revoked); err != nil {
			t.Fatalf("query revoke state for %s: %v", id, err)
		}
		if !revoked.Valid {
			t.Errorf("row %s should be revoked after RevokeFamily", id)
		}
	}
}

func openRefreshAssertRevoked(t *testing.T, r *RefreshRepo, id string, out *sql.NullString) error {
	t.Helper()
	return r.mainDB.QueryRow(`SELECT revoked_at FROM oauth_refresh_tokens WHERE id = ?`, id).Scan(out)
}
