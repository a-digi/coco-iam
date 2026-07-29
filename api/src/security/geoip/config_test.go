package geoip

import (
	"testing"
	"time"
)

func TestLoadConfig_DefaultsWhenGeoipKeyAbsent(t *testing.T) {
	cfg, err := LoadConfig([]byte(`{"port": 2026}`))
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	want := DefaultConfig()
	if cfg != want {
		t.Errorf("cfg = %+v, want default %+v", cfg, want)
	}
}

func TestLoadConfig_PartialOverrideKeepsOtherDefaults(t *testing.T) {
	cfg, err := LoadConfig([]byte(`{"geoip": {"check_interval_seconds": 120}}`))
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.CheckIntervalSeconds != 120 {
		t.Errorf("CheckIntervalSeconds = %d, want 120", cfg.CheckIntervalSeconds)
	}
	want := DefaultConfig()
	if cfg.PullIntervalHours != want.PullIntervalHours {
		t.Errorf("PullIntervalHours should keep its default when omitted, got %d", cfg.PullIntervalHours)
	}
	if cfg.DBPath != want.DBPath {
		t.Errorf("DBPath should keep its default when omitted, got %q", cfg.DBPath)
	}
	if cfg.Enabled {
		t.Error("Enabled should keep its default (false) when omitted")
	}
}

func TestLoadConfig_FullOverride(t *testing.T) {
	cfg, err := LoadConfig([]byte(`{
		"geoip": {
			"enabled": true,
			"db_path": "/tmp/geoip.db",
			"check_interval_seconds": 300,
			"pull_interval_hours": 12,
			"maxmind_account_id": "acct-1",
			"maxmind_license_key": "key-1",
			"pid_file": "/tmp/geoip-updater.pid",
			"updater_binary_path": "/opt/coco-iam/geoipupdater"
		}
	}`))
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	want := Config{
		Enabled:              true,
		DBPath:               "/tmp/geoip.db",
		CheckIntervalSeconds: 300,
		PullIntervalHours:    12,
		MaxMindAccountID:     "acct-1",
		MaxMindLicenseKey:    "key-1",
		PIDFile:              "/tmp/geoip-updater.pid",
		UpdaterBinaryPath:    "/opt/coco-iam/geoipupdater",
	}
	if cfg != want {
		t.Errorf("cfg = %+v, want %+v", cfg, want)
	}
}

func TestLoadConfig_InvalidJSON(t *testing.T) {
	if _, err := LoadConfig([]byte(`{not json`)); err == nil {
		t.Fatal("expected an error for malformed JSON")
	}
}

func TestLoadConfig_DisabledSkipsValidation(t *testing.T) {
	// enabled: false (the default) must never fail to boot just
	// because credentials/paths haven't been configured yet.
	if _, err := LoadConfig([]byte(`{"geoip": {"enabled": false}}`)); err != nil {
		t.Fatalf("LoadConfig() error = %v, want no error when disabled", err)
	}
}

func TestLoadConfig_RejectsEnabledWithoutDBPath(t *testing.T) {
	_, err := LoadConfig([]byte(`{"geoip": {"enabled": true, "db_path": "", "check_interval_seconds": 600, "pull_interval_hours": 24, "maxmind_account_id": "a", "maxmind_license_key": "b"}}`))
	if err == nil {
		t.Fatal("expected an error for enabled=true with empty db_path")
	}
}

func TestLoadConfig_RejectsEnabledWithZeroCheckInterval(t *testing.T) {
	_, err := LoadConfig([]byte(`{"geoip": {"enabled": true, "db_path": "x", "check_interval_seconds": 0, "pull_interval_hours": 24, "maxmind_account_id": "a", "maxmind_license_key": "b"}}`))
	if err == nil {
		t.Fatal("expected an error for enabled=true with check_interval_seconds <= 0")
	}
}

func TestLoadConfig_RejectsEnabledWithZeroPullInterval(t *testing.T) {
	_, err := LoadConfig([]byte(`{"geoip": {"enabled": true, "db_path": "x", "check_interval_seconds": 600, "pull_interval_hours": 0, "maxmind_account_id": "a", "maxmind_license_key": "b"}}`))
	if err == nil {
		t.Fatal("expected an error for enabled=true with pull_interval_hours <= 0")
	}
}

func TestLoadConfig_RejectsEnabledWithoutCredentials(t *testing.T) {
	_, err := LoadConfig([]byte(`{"geoip": {"enabled": true, "db_path": "x", "check_interval_seconds": 600, "pull_interval_hours": 24}}`))
	if err == nil {
		t.Fatal("expected an error for enabled=true with no MaxMind credentials")
	}
}

func TestLoadConfig_RejectsEnabledWithoutPIDFile(t *testing.T) {
	_, err := LoadConfig([]byte(`{"geoip": {"enabled": true, "db_path": "x", "check_interval_seconds": 600, "pull_interval_hours": 24, "maxmind_account_id": "a", "maxmind_license_key": "b", "pid_file": "", "updater_binary_path": "y"}}`))
	if err == nil {
		t.Fatal("expected an error for enabled=true with empty pid_file")
	}
}

func TestLoadConfig_RejectsEnabledWithoutUpdaterBinaryPath(t *testing.T) {
	_, err := LoadConfig([]byte(`{"geoip": {"enabled": true, "db_path": "x", "check_interval_seconds": 600, "pull_interval_hours": 24, "maxmind_account_id": "a", "maxmind_license_key": "b", "pid_file": "y", "updater_binary_path": ""}}`))
	if err == nil {
		t.Fatal("expected an error for enabled=true with empty updater_binary_path")
	}
}

func TestConfig_CheckIntervalConvertsSecondsToDuration(t *testing.T) {
	cfg := Config{CheckIntervalSeconds: 600}
	if got := cfg.CheckInterval(); got != 10*time.Minute {
		t.Errorf("CheckInterval() = %v, want 10m", got)
	}
}

func TestConfig_PullIntervalConvertsHoursToDuration(t *testing.T) {
	cfg := Config{PullIntervalHours: 24}
	if got := cfg.PullInterval(); got != 24*time.Hour {
		t.Errorf("PullInterval() = %v, want 24h", got)
	}
}
