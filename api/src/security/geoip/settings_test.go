package geoip

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// geoipSettingsSchema mirrors
// api/config/db/migrations/29_07_2026_09_00_00.sql exactly — inlined
// here rather than read from disk so this test doesn't depend on the
// working directory a test binary happens to run from, same approach
// every other test file in this codebase uses for a real schema.
const geoipSettingsSchema = `
CREATE TABLE IF NOT EXISTS geoip_settings
(
    id                     INTEGER NOT NULL PRIMARY KEY CHECK (id = 1),
    enabled                INTEGER NOT NULL DEFAULT 0,
    maxmind_account_id     TEXT NOT NULL DEFAULT '',
    maxmind_license_key    TEXT NOT NULL DEFAULT '',
    check_interval_seconds INTEGER NOT NULL DEFAULT 600,
    pull_interval_hours    INTEGER NOT NULL DEFAULT 24,
    updated_at             DATETIME
);
`

func newSettingsTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(geoipSettingsSchema); err != nil {
		t.Fatalf("apply schema: %v", err)
	}
	return db
}

func TestSettingsQueryRepo_LoadSettings_ReturnsZeroValueWhenNoRowExists(t *testing.T) {
	db := newSettingsTestDB(t)
	query := NewSettingsQueryRepo(db)

	s, err := query.LoadSettings()
	if err != nil {
		t.Fatalf("LoadSettings() error = %v", err)
	}
	if s.loaded {
		t.Fatal("LoadSettings().loaded = true, want false when no row has ever been saved")
	}
	if s != (Settings{}) {
		t.Fatalf("LoadSettings() = %+v, want the zero value", s)
	}
}

func TestSettingsPersistentRepo_SaveSettings_ThenLoadable(t *testing.T) {
	db := newSettingsTestDB(t)
	persist := NewSettingsPersistentRepo(db)
	query := NewSettingsQueryRepo(db)

	want := Settings{
		Enabled:              true,
		MaxMindAccountID:     "acct-1",
		MaxMindLicenseKey:    "secret-key",
		CheckIntervalSeconds: 300,
		PullIntervalHours:    12,
	}
	if err := persist.SaveSettings(want); err != nil {
		t.Fatalf("SaveSettings() error = %v", err)
	}

	got, err := query.LoadSettings()
	if err != nil {
		t.Fatalf("LoadSettings() error = %v", err)
	}
	if !got.loaded {
		t.Fatal("LoadSettings().loaded = false, want true after a save")
	}
	if got.Enabled != want.Enabled || got.MaxMindAccountID != want.MaxMindAccountID ||
		got.MaxMindLicenseKey != want.MaxMindLicenseKey || got.CheckIntervalSeconds != want.CheckIntervalSeconds ||
		got.PullIntervalHours != want.PullIntervalHours {
		t.Fatalf("LoadSettings() = %+v, want fields matching %+v", got, want)
	}
	if got.UpdatedAt.IsZero() {
		t.Fatal("LoadSettings().UpdatedAt should be set after a save")
	}
}

func TestSettingsPersistentRepo_SaveSettings_UpsertsInPlace(t *testing.T) {
	db := newSettingsTestDB(t)
	persist := NewSettingsPersistentRepo(db)
	query := NewSettingsQueryRepo(db)

	if err := persist.SaveSettings(Settings{Enabled: false, MaxMindAccountID: "first", MaxMindLicenseKey: "key-1", CheckIntervalSeconds: 600, PullIntervalHours: 24}); err != nil {
		t.Fatalf("first SaveSettings() error = %v", err)
	}
	if err := persist.SaveSettings(Settings{Enabled: true, MaxMindAccountID: "second", MaxMindLicenseKey: "key-2", CheckIntervalSeconds: 300, PullIntervalHours: 12}); err != nil {
		t.Fatalf("second SaveSettings() error = %v", err)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM geoip_settings`).Scan(&count); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("geoip_settings row count = %d, want exactly 1 (upsert, not a second row)", count)
	}

	got, err := query.LoadSettings()
	if err != nil {
		t.Fatalf("LoadSettings() error = %v", err)
	}
	if got.MaxMindAccountID != "second" || got.MaxMindLicenseKey != "key-2" || !got.Enabled {
		t.Fatalf("LoadSettings() = %+v, want the second save's values in place", got)
	}
}

func TestSettingsPersistentRepo_SaveSettings_EmptyLicenseKeyDoesNotOverwriteExisting(t *testing.T) {
	db := newSettingsTestDB(t)
	persist := NewSettingsPersistentRepo(db)
	query := NewSettingsQueryRepo(db)

	if err := persist.SaveSettings(Settings{MaxMindAccountID: "acct-1", MaxMindLicenseKey: "original-key", CheckIntervalSeconds: 600, PullIntervalHours: 24}); err != nil {
		t.Fatalf("first SaveSettings() error = %v", err)
	}

	// A later save with an empty license key — e.g. the admin only
	// changed the account ID and left the masked key field blank —
	// must not wipe the previously-stored key.
	if err := persist.SaveSettings(Settings{MaxMindAccountID: "acct-2", MaxMindLicenseKey: "", CheckIntervalSeconds: 600, PullIntervalHours: 24}); err != nil {
		t.Fatalf("second SaveSettings() error = %v", err)
	}

	got, err := query.LoadSettings()
	if err != nil {
		t.Fatalf("LoadSettings() error = %v", err)
	}
	if got.MaxMindAccountID != "acct-2" {
		t.Fatalf("MaxMindAccountID = %q, want acct-2 (should still update)", got.MaxMindAccountID)
	}
	if got.MaxMindLicenseKey != "original-key" {
		t.Fatalf("MaxMindLicenseKey = %q, want original-key preserved (empty input must not overwrite)", got.MaxMindLicenseKey)
	}
}

func TestConfig_WithSettings_OverridesFieldsWhenLoaded(t *testing.T) {
	base := DefaultConfig()
	loaded := Settings{
		loaded:               true,
		Enabled:              true,
		MaxMindAccountID:     "acct-1",
		MaxMindLicenseKey:    "key-1",
		CheckIntervalSeconds: 120,
		PullIntervalHours:    6,
	}

	merged := base.WithSettings(loaded)
	if !merged.Enabled || merged.MaxMindAccountID != "acct-1" || merged.MaxMindLicenseKey != "key-1" ||
		merged.CheckIntervalSeconds != 120 || merged.PullIntervalHours != 6 {
		t.Fatalf("WithSettings() = %+v, want overridden fields from settings", merged)
	}
	// DBPath is not part of Settings — must survive untouched.
	if merged.DBPath != base.DBPath {
		t.Errorf("DBPath = %q, want unchanged %q", merged.DBPath, base.DBPath)
	}
}

func TestConfig_WithSettings_NoopWhenSettingsNeverSaved(t *testing.T) {
	base := DefaultConfig()
	merged := base.WithSettings(Settings{})
	if merged != base {
		t.Fatalf("WithSettings(zero value) = %+v, want unchanged %+v", merged, base)
	}
}
