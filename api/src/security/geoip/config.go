package geoip

import (
	"encoding/json"
	"fmt"
	"time"
)

// Config is the "geoip" section of config.json. It is parsed locally
// by this package, not added to coco-server's Config struct — same
// pattern already used for the existing "auth" and "security" keys
// (see ipguard.Config's own doc comment). Shared verbatim by both the
// main coco-iam server (read side: SQLLookup/Watcher) and the
// separate geoip-updater executable (write side) — both processes
// must agree on DBPath, and only the updater needs the MaxMind
// credentials.
type Config struct {
	// Enabled gates everything: if false, the caller wires a
	// NoopLookup and never constructs SQLLookup/Watcher/updater at
	// all — the safe default for a fresh checkout with no MaxMind
	// account configured yet.
	Enabled bool `json:"enabled"`
	// DBPath is where the read side expects to find geoip.db, and
	// where the write side atomically renames a freshly-built
	// replacement into place. See plan/geoip-enrichment/plan.md.
	DBPath string `json:"db_path"`
	// CheckIntervalSeconds is how often the read-side Watcher polls
	// DBPath's mtime for a change, and, independently, how often the
	// updater's own loop wakes up to ask "is it time to pull yet" —
	// explicitly NOT how often data is actually re-downloaded. See
	// PullIntervalHours.
	CheckIntervalSeconds int `json:"check_interval_seconds"`
	// PullIntervalHours is how stale the last successful pull must be
	// before the updater actually re-downloads and re-imports. Kept
	// well above the check interval — MaxMind only republishes
	// GeoLite2 twice a week, so pulling more often than this wastes
	// bandwidth for no fresher data.
	PullIntervalHours int `json:"pull_interval_hours"`
	// MaxMindAccountID and MaxMindLicenseKey authenticate the
	// updater's download requests (HTTP Basic auth). Free tier,
	// requires a free MaxMind account. Unused by the read side.
	MaxMindAccountID  string `json:"maxmind_account_id"`
	MaxMindLicenseKey string `json:"maxmind_license_key"`
	// PIDFile is where the updater process writes its own PID on
	// startup and removes it on clean exit — the same
	// WritePID/ReadPID/RemovePID mechanism main.go already uses on
	// itself (vendor/github.com/a-digi/coco-server/server/pid.go).
	// Read by geoip.Manager (main server side) to find and signal the
	// process; written by cmd/geoipupdater itself (updater side). See
	// plan/geoip-enrichment/plan.md's "Process control" section.
	PIDFile string `json:"pid_file"`
	// UpdaterBinaryPath is where geoip.Manager execs the geoipupdater
	// binary from — only meaningful on the main server side (the
	// updater process itself never reads this field). Never taken
	// from request input; always this static, operator-set path.
	UpdaterBinaryPath string `json:"updater_binary_path"`
}

// CheckInterval converts CheckIntervalSeconds to a time.Duration —
// every caller wants a Duration, not a bare int, and doing the
// conversion here keeps the seconds-vs-hours distinction out of every
// call site.
func (c Config) CheckInterval() time.Duration {
	return time.Duration(c.CheckIntervalSeconds) * time.Second
}

// PullInterval converts PullIntervalHours to a time.Duration.
func (c Config) PullInterval() time.Duration {
	return time.Duration(c.PullIntervalHours) * time.Hour
}

// DefaultConfig returns the starting values from
// plan/geoip-enrichment/plan.md — disabled by default, none of this
// hardcoded into any lookup/update logic itself.
func DefaultConfig() Config {
	return Config{
		Enabled:              false,
		DBPath:               "./data/db/security/geoip.db",
		CheckIntervalSeconds: 600,
		PullIntervalHours:    24,
		PIDFile:              "./data/geoip-updater.pid",
		UpdaterBinaryPath:    "./versions/geoipupdater",
	}
}

// LoadConfig parses the "geoip" key out of raw config.json bytes.
// Fields absent from cfgBytes (including the whole "geoip" key) keep
// their DefaultConfig() value — this lets an operator override just
// one field without having to restate the whole block.
func LoadConfig(cfgBytes []byte) (Config, error) {
	cfg := DefaultConfig()
	wrapper := struct {
		GeoIP *Config `json:"geoip"`
	}{GeoIP: &cfg}

	if err := json.Unmarshal(cfgBytes, &wrapper); err != nil {
		return Config{}, fmt.Errorf("could not parse geoip config: %w", err)
	}

	if err := wrapper.GeoIP.validate(); err != nil {
		return Config{}, err
	}

	return *wrapper.GeoIP, nil
}

// validate only enforces anything when Enabled is true — a disabled
// config (the default) is always valid regardless of what else is
// set, so a fresh checkout never fails to boot over geoip.
func (c Config) validate() error {
	if !c.Enabled {
		return nil
	}
	if c.DBPath == "" {
		return fmt.Errorf("geoip.db_path must be set when geoip.enabled is true")
	}
	if c.CheckIntervalSeconds <= 0 {
		return fmt.Errorf("geoip.check_interval_seconds must be > 0, got %d", c.CheckIntervalSeconds)
	}
	if c.PullIntervalHours <= 0 {
		return fmt.Errorf("geoip.pull_interval_hours must be > 0, got %d", c.PullIntervalHours)
	}
	if c.MaxMindAccountID == "" || c.MaxMindLicenseKey == "" {
		return fmt.Errorf("geoip.maxmind_account_id and geoip.maxmind_license_key must be set when geoip.enabled is true")
	}
	if c.PIDFile == "" {
		return fmt.Errorf("geoip.pid_file must be set when geoip.enabled is true")
	}
	if c.UpdaterBinaryPath == "" {
		return fmt.Errorf("geoip.updater_binary_path must be set when geoip.enabled is true")
	}
	return nil
}
