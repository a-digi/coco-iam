package general

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// freshDB creates an in-memory SQLite DB with the app_settings schema
// that the per-org migration (22_app_settings.sql) produces.
func freshDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`
		CREATE TABLE app_settings (
			id      TEXT NOT NULL PRIMARY KEY UNIQUE,
			key     TEXT NOT NULL,
			value   TEXT NOT NULL DEFAULT '',
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE UNIQUE INDEX app_settings_key_uindex ON app_settings (key);
	`)
	if err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return db
}

func freshStore(t *testing.T) *Store {
	t.Helper()
	return NewStoreFromDB(freshDB(t))
}

func TestStore_GetMissing(t *testing.T) {
	s := freshStore(t)
	_, found, err := s.Get("general.base_url")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Error("expected key to be absent")
	}
}

func TestStore_SetAndGet(t *testing.T) {
	s := freshStore(t)
	if err := s.Set(KeyBaseURL, "https://example.com"); err != nil {
		t.Fatalf("set: %v", err)
	}
	v, found, err := s.Get(KeyBaseURL)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !found {
		t.Fatal("key should be present after Set")
	}
	if v != "https://example.com" {
		t.Errorf("got %q, want https://example.com", v)
	}
}

func TestStore_SetUpserts(t *testing.T) {
	s := freshStore(t)
	_ = s.Set(KeyBaseURL, "https://old.example.com")
	if err := s.Set(KeyBaseURL, "https://new.example.com"); err != nil {
		t.Fatalf("second set: %v", err)
	}
	v, _, _ := s.Get(KeyBaseURL)
	if v != "https://new.example.com" {
		t.Errorf("expected updated value, got %q", v)
	}
}

func TestStore_Delete(t *testing.T) {
	s := freshStore(t)
	_ = s.Set(KeyBaseURL, "https://example.com")
	if err := s.Delete(KeyBaseURL); err != nil {
		t.Fatalf("delete: %v", err)
	}
	_, found, _ := s.Get(KeyBaseURL)
	if found {
		t.Error("key should be absent after Delete")
	}
}

func TestStore_BaseURL_TrimsTrailingSlash(t *testing.T) {
	s := freshStore(t)
	_ = s.Set(KeyBaseURL, "https://example.com/")
	got := s.BaseURL()
	if got != "https://example.com" {
		t.Errorf("got %q, want trailing slash stripped", got)
	}
}

func TestStore_BaseURL_EmptyWhenUnset(t *testing.T) {
	s := freshStore(t)
	if s.BaseURL() != "" {
		t.Error("BaseURL should be empty when key not set")
	}
}

func TestStore_PageTitle_EmptyWhenUnset(t *testing.T) {
	s := freshStore(t)
	if s.PageTitle() != "" {
		t.Error("PageTitle should be empty when key not set")
	}
}

func TestStore_PageTitle_ReturnsValue(t *testing.T) {
	s := freshStore(t)
	_ = s.Set(KeyPageTitle, "  My App  ")
	if got := s.PageTitle(); got != "My App" {
		t.Errorf("got %q, want trimmed value", got)
	}
}

func TestStore_Snapshot_AllFields(t *testing.T) {
	s := freshStore(t)
	_ = s.Set(KeyBaseURL, "https://example.com")
	_ = s.Set(KeyPageTitle, "My App")
	_ = s.Set(KeyDescription, "A great app")
	_ = s.Set(KeyRobots, "index, follow")

	snap, err := s.Snapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if snap.BaseURL != "https://example.com" {
		t.Errorf("BaseURL: got %q", snap.BaseURL)
	}
	if snap.PageTitle != "My App" {
		t.Errorf("PageTitle: got %q", snap.PageTitle)
	}
	if snap.Description != "A great app" {
		t.Errorf("Description: got %q", snap.Description)
	}
	if snap.Robots != "index, follow" {
		t.Errorf("Robots: got %q", snap.Robots)
	}
}

func TestStore_Snapshot_EmptyWhenNothingSet(t *testing.T) {
	s := freshStore(t)
	snap, err := s.Snapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if snap.BaseURL != "" || snap.PageTitle != "" || snap.Description != "" || snap.Robots != "" {
		t.Errorf("expected empty snapshot, got %+v", snap)
	}
}

func TestStore_SetMany(t *testing.T) {
	s := freshStore(t)
	err := s.SetMany(map[string]string{
		KeyBaseURL:   "https://example.com",
		KeyPageTitle: "Test",
	})
	if err != nil {
		t.Fatalf("SetMany: %v", err)
	}
	if s.BaseURL() != "https://example.com" {
		t.Error("base_url not set by SetMany")
	}
	if s.PageTitle() != "Test" {
		t.Error("page_title not set by SetMany")
	}
}
