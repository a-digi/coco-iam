package recovery

import (
	"database/sql"
	"testing"
	"time"

	mailsettings "github.com/a-digi/coco-iam/src/mail/settings"
	_ "github.com/mattn/go-sqlite3"
)

func freshMailStore(t *testing.T) *mailsettings.Store {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`
		CREATE TABLE mail_settings (
			key        TEXT PRIMARY KEY,
			value      TEXT NOT NULL DEFAULT '',
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return mailsettings.NewStoreFromDB(db)
}

func freshReader(t *testing.T) (*SettingsReader, *mailsettings.Store) {
	t.Helper()
	ms := freshMailStore(t)
	return NewSettingsReader(ms), ms
}

func TestSettingsReader_TTL_DefaultWhenUnset(t *testing.T) {
	r, _ := freshReader(t)
	got := r.TTL()
	want := time.Duration(defaultTTLHours) * time.Hour
	if got != want {
		t.Errorf("TTL: got %v, want %v", got, want)
	}
}

func TestSettingsReader_TTL_ReadsFromStore(t *testing.T) {
	r, ms := freshReader(t)
	if err := ms.Set(KeyTTLHours, "3"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if got := r.TTL(); got != 3*time.Hour {
		t.Errorf("TTL: got %v, want 3h", got)
	}
}

func TestSettingsReader_TTL_ZeroValueUsesDefault(t *testing.T) {
	r, ms := freshReader(t)
	if err := ms.Set(KeyTTLHours, "0"); err != nil {
		t.Fatalf("set: %v", err)
	}
	got := r.TTL()
	want := time.Duration(defaultTTLHours) * time.Hour
	if got != want {
		t.Errorf("TTL: zero value should fall back to default, got %v", got)
	}
}

func TestSettingsReader_ResendCooldown_DefaultWhenUnset(t *testing.T) {
	r, _ := freshReader(t)
	got := r.ResendCooldown()
	want := time.Duration(defaultResendCooldownSec) * time.Second
	if got != want {
		t.Errorf("ResendCooldown: got %v, want %v", got, want)
	}
}

func TestSettingsReader_ResendCooldown_ReadsFromStore(t *testing.T) {
	r, ms := freshReader(t)
	if err := ms.Set(KeyResendCooldownSeconds, "120"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if got := r.ResendCooldown(); got != 120*time.Second {
		t.Errorf("ResendCooldown: got %v, want 120s", got)
	}
}

func TestSettingsReader_ResendCooldown_ZeroIsAllowed(t *testing.T) {
	r, ms := freshReader(t)
	if err := ms.Set(KeyResendCooldownSeconds, "0"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if got := r.ResendCooldown(); got != 0 {
		t.Errorf("ResendCooldown: got %v, want 0 (zero is valid for tests)", got)
	}
}

func TestSettingsReader_TTLHumanReadable_Singular(t *testing.T) {
	r, ms := freshReader(t)
	_ = ms.Set(KeyTTLHours, "1")
	if got := r.TTLHumanReadable(); got != "1 hour" {
		t.Errorf("got %q, want '1 hour'", got)
	}
}

func TestSettingsReader_TTLHumanReadable_Plural(t *testing.T) {
	r, ms := freshReader(t)
	_ = ms.Set(KeyTTLHours, "2")
	if got := r.TTLHumanReadable(); got != "2 hours" {
		t.Errorf("got %q, want '2 hours'", got)
	}
}
