package repository

import (
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/a-digi/coco-iam/src/admin/users/profile/entity"
	_ "github.com/mattn/go-sqlite3"
)

func freshRepo(t *testing.T) *Repository {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`
		CREATE TABLE admin_user_profiles (
			admin_user_id   TEXT NOT NULL PRIMARY KEY,
			first_name      TEXT NOT NULL DEFAULT '',
			last_name       TEXT NOT NULL DEFAULT '',
			phone           TEXT NOT NULL DEFAULT '',
			avatar_asset_id TEXT NOT NULL DEFAULT '',
			locale          TEXT NOT NULL DEFAULT '',
			timezone        TEXT NOT NULL DEFAULT '',
			created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return New(db)
}

func TestFindByAdminUserID_MissingReturnsErrNotFound(t *testing.T) {
	// Lazy-create semantics: the handler treats ErrNotFound as "no
	// profile row yet, return empty projection". That behaviour
	// hinges on this exact sentinel.
	repo := freshRepo(t)
	_, err := repo.FindByAdminUserID("missing")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestUpsert_CreatesThenReplacesAndPreservesCreatedAt(t *testing.T) {
	// Pin the critical invariant: repeat upserts only bump
	// updated_at, never rewrite created_at. Clients displaying
	// "member since" rely on this stability.
	repo := freshRepo(t)
	p := &entity.AdminUserProfile{
		AdminUserID: "user-1", FirstName: "Alice", LastName: "P",
	}
	if err := repo.Upsert(p); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	got1, err := repo.FindByAdminUserID("user-1")
	if err != nil {
		t.Fatalf("find 1: %v", err)
	}
	if got1.FirstName != "Alice" {
		t.Errorf("first_name round-trip failed: %q", got1.FirstName)
	}

	// SQLite's CURRENT_TIMESTAMP has 1-second resolution — wait so
	// a change in updated_at is observable.
	time.Sleep(1100 * time.Millisecond)

	p.FirstName = "Alicia"
	if err := repo.Upsert(p); err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	got2, err := repo.FindByAdminUserID("user-1")
	if err != nil {
		t.Fatalf("find 2: %v", err)
	}
	if got2.FirstName != "Alicia" {
		t.Errorf("first_name update failed: %q", got2.FirstName)
	}
	if !got2.CreatedAt.Equal(got1.CreatedAt) {
		t.Errorf("created_at should be preserved across upserts: was %v, now %v",
			got1.CreatedAt, got2.CreatedAt)
	}
	if !got2.UpdatedAt.After(got1.UpdatedAt) {
		t.Errorf("updated_at should advance on every upsert: was %v, now %v",
			got1.UpdatedAt, got2.UpdatedAt)
	}
}

func TestUpsert_RequiresAdminUserID(t *testing.T) {
	repo := freshRepo(t)
	err := repo.Upsert(&entity.AdminUserProfile{FirstName: "A"})
	if err == nil {
		t.Fatal("empty admin_user_id must be rejected")
	}
}

func TestUpdateAvatarAssetID_CreatesRowIfMissing(t *testing.T) {
	// Avatar upload is often the first thing an admin does — the
	// profile row may not exist yet. UpdateAvatarAssetID has to
	// lazy-create rather than error; otherwise the upload handler
	// has to juggle "upsert empty profile, then set avatar".
	repo := freshRepo(t)
	if err := repo.UpdateAvatarAssetID("user-1", "user-1.png"); err != nil {
		t.Fatalf("first upload on missing profile: %v", err)
	}
	got, err := repo.FindByAdminUserID("user-1")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got.AvatarAssetID != "user-1.png" {
		t.Errorf("avatar_asset_id: want user-1.png, got %q", got.AvatarAssetID)
	}
	// Other fields must be empty strings, not NULL.
	if got.FirstName != "" || got.LastName != "" || got.Phone != "" {
		t.Errorf("other fields should default to empty: %+v", got)
	}
}

func TestUpdateAvatarAssetID_DoesNotWipeOtherFields(t *testing.T) {
	// Writing just the avatar must leave first_name/last_name etc.
	// untouched. Otherwise a user who uploads a new avatar would
	// silently lose their other profile data.
	repo := freshRepo(t)
	if err := repo.Upsert(&entity.AdminUserProfile{
		AdminUserID: "user-1", FirstName: "Alice", LastName: "P", Phone: "+49",
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := repo.UpdateAvatarAssetID("user-1", "user-1.webp"); err != nil {
		t.Fatalf("update avatar: %v", err)
	}
	got, _ := repo.FindByAdminUserID("user-1")
	if got.FirstName != "Alice" || got.LastName != "P" || got.Phone != "+49" {
		t.Errorf("other fields should survive: %+v", got)
	}
	if got.AvatarAssetID != "user-1.webp" {
		t.Errorf("avatar_asset_id: want user-1.webp, got %q", got.AvatarAssetID)
	}
}

func TestClearAvatar_EmptyString(t *testing.T) {
	repo := freshRepo(t)
	if err := repo.UpdateAvatarAssetID("user-1", "user-1.png"); err != nil {
		t.Fatalf("seed avatar: %v", err)
	}
	if err := repo.ClearAvatar("user-1"); err != nil {
		t.Fatalf("clear: %v", err)
	}
	got, _ := repo.FindByAdminUserID("user-1")
	if got.AvatarAssetID != "" {
		t.Errorf("after clear: avatar_asset_id should be empty, got %q", got.AvatarAssetID)
	}
}
