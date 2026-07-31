package activation

import (
	"database/sql"
	"testing"
	"time"

	notsettings "github.com/a-digi/coco-notification/settings"
	_ "github.com/mattn/go-sqlite3"
)

// freshMailStore returns a SettingsReader backed by an in-memory
// notification_settings SQLite DB. The schema mirrors what the real
// notification_settings store expects.
func freshMailStore(t *testing.T) *notsettings.Store {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`
		CREATE TABLE notification_settings (
			key        TEXT PRIMARY KEY,
			value      TEXT NOT NULL DEFAULT '',
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return notsettings.NewStoreFromDB(db)
}

func freshReader(t *testing.T) (*SettingsReader, *notsettings.Store) {
	t.Helper()
	ms := freshMailStore(t)
	return NewSettingsReader(ms), ms
}

func TestSettingsReader_TTL_DefaultIsUsedWhenUnset(t *testing.T) {
	r, _ := freshReader(t)
	got := r.TTL()
	want := time.Duration(defaultTTLHours) * time.Hour
	if got != want {
		t.Errorf("TTL: got %v, want %v", got, want)
	}
}

func TestSettingsReader_TTL_ReadsFromStore(t *testing.T) {
	r, ms := freshReader(t)
	if err := ms.Set(KeyTTLHours, "48"); err != nil {
		t.Fatalf("set: %v", err)
	}
	got := r.TTL()
	want := 48 * time.Hour
	if got != want {
		t.Errorf("TTL: got %v, want %v", got, want)
	}
}

func TestSettingsReader_TTL_InvalidValueUsesDefault(t *testing.T) {
	r, ms := freshReader(t)
	if err := ms.Set(KeyTTLHours, "not-a-number"); err != nil {
		t.Fatalf("set: %v", err)
	}
	got := r.TTL()
	want := time.Duration(defaultTTLHours) * time.Hour
	if got != want {
		t.Errorf("TTL: got %v, want default %v", got, want)
	}
}

func TestSettingsReader_ResendCooldown_DefaultIsUsedWhenUnset(t *testing.T) {
	r, _ := freshReader(t)
	got := r.ResendCooldown()
	want := time.Duration(defaultResendCooldownSec) * time.Second
	if got != want {
		t.Errorf("ResendCooldown: got %v, want %v", got, want)
	}
}

func TestSettingsReader_ResendCooldown_ReadsFromStore(t *testing.T) {
	r, ms := freshReader(t)
	if err := ms.Set(KeyResendCooldownSeconds, "60"); err != nil {
		t.Fatalf("set: %v", err)
	}
	got := r.ResendCooldown()
	want := 60 * time.Second
	if got != want {
		t.Errorf("ResendCooldown: got %v, want %v", got, want)
	}
}

func TestSettingsReader_TTLHumanReadable_Singular(t *testing.T) {
	r, ms := freshReader(t)
	if err := ms.Set(KeyTTLHours, "1"); err != nil {
		t.Fatalf("set: %v", err)
	}
	got := r.TTLHumanReadable()
	if got != "1 hour" {
		t.Errorf("got %q, want '1 hour'", got)
	}
}

func TestSettingsReader_TTLHumanReadable_Plural(t *testing.T) {
	r, ms := freshReader(t)
	if err := ms.Set(KeyTTLHours, "24"); err != nil {
		t.Fatalf("set: %v", err)
	}
	got := r.TTLHumanReadable()
	if got != "24 hours" {
		t.Errorf("got %q, want '24 hours'", got)
	}
}
