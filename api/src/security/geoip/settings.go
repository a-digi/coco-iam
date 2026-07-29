package geoip

import (
	"database/sql"
	"fmt"
	"time"
)

// Settings is the admin-editable subset of geoip configuration,
// stored in the main database's geoip_settings singleton row rather
// than config.json — see plan/geoip-enrichment/plan.md's
// "Admin-editable settings" section for why: geoip.db itself is
// rebuilt wholesale and atomically swapped on every successful pull
// (no history kept, by design), so anything stored there would be
// silently wiped the next time the updater runs. Settings therefore
// live in the same main database ip_bans/ip_allowlist already do.
type Settings struct {
	Enabled              bool
	MaxMindAccountID     string
	MaxMindLicenseKey    string
	CheckIntervalSeconds int
	PullIntervalHours    int
	UpdatedAt            time.Time

	// loaded is true only when this Settings came from an actual
	// stored row — unexported, since it's purely an internal signal
	// for WithSettings below, never something a caller outside this
	// package needs to see.
	loaded bool
}

// WithSettings returns a copy of c with the admin-editable fields
// (Enabled, MaxMindAccountID, MaxMindLicenseKey, CheckIntervalSeconds,
// PullIntervalHours) overridden by s. If s is the zero value — no
// settings row has ever been saved, e.g. a fresh install nobody has
// configured yet — this is a no-op, leaving c's own static config.json
// defaults in place.
func (c Config) WithSettings(s Settings) Config {
	if !s.loaded {
		return c
	}
	c.Enabled = s.Enabled
	c.MaxMindAccountID = s.MaxMindAccountID
	c.MaxMindLicenseKey = s.MaxMindLicenseKey
	c.CheckIntervalSeconds = s.CheckIntervalSeconds
	c.PullIntervalHours = s.PullIntervalHours
	return c
}

// SettingsQueryRepo reads the geoip_settings singleton row from the
// main database — the same *sql.DB ipguard's ban/allowlist repos
// already use, not geoip.db (see Settings' own doc comment for why).
type SettingsQueryRepo struct {
	db *sql.DB
}

func NewSettingsQueryRepo(db *sql.DB) *SettingsQueryRepo {
	return &SettingsQueryRepo{db: db}
}

// LoadSettings returns the current settings, or the zero value (never
// an error) if no row has ever been saved — a fresh install with
// nobody having visited the settings page yet is an expected state,
// not a failure.
func (r *SettingsQueryRepo) LoadSettings() (Settings, error) {
	var s Settings
	var enabled int
	var updatedAt sql.NullString
	err := r.db.QueryRow(
		`SELECT enabled, maxmind_account_id, maxmind_license_key, check_interval_seconds, pull_interval_hours, updated_at
		 FROM geoip_settings WHERE id = 1`,
	).Scan(&enabled, &s.MaxMindAccountID, &s.MaxMindLicenseKey, &s.CheckIntervalSeconds, &s.PullIntervalHours, &updatedAt)
	if err == sql.ErrNoRows {
		return Settings{}, nil
	}
	if err != nil {
		return Settings{}, fmt.Errorf("geoip: load settings: %w", err)
	}
	s.Enabled = enabled != 0
	s.loaded = true
	if updatedAt.Valid {
		s.UpdatedAt = parseSettingsTime(updatedAt.String)
	}
	return s, nil
}

// SettingsPersistentRepo writes the geoip_settings singleton row.
type SettingsPersistentRepo struct {
	db *sql.DB
}

func NewSettingsPersistentRepo(db *sql.DB) *SettingsPersistentRepo {
	return &SettingsPersistentRepo{db: db}
}

// SaveSettings upserts the singleton row. An empty MaxMindLicenseKey
// means "leave the stored key unchanged" — the admin PUT handler's
// contract (see plan/geoip-enrichment/plan.md), enforced here at the
// SQL level via a CASE against the existing value rather than left to
// handler-layer discipline, so the invariant holds regardless of
// caller: this repository can never be made to wipe a previously-saved
// key with a blank one.
func (r *SettingsPersistentRepo) SaveSettings(s Settings) error {
	enabled := 0
	if s.Enabled {
		enabled = 1
	}
	_, err := r.db.Exec(
		`INSERT INTO geoip_settings (id, enabled, maxmind_account_id, maxmind_license_key, check_interval_seconds, pull_interval_hours, updated_at)
		 VALUES (1, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(id) DO UPDATE SET
		   enabled = excluded.enabled,
		   maxmind_account_id = excluded.maxmind_account_id,
		   maxmind_license_key = CASE WHEN excluded.maxmind_license_key = '' THEN geoip_settings.maxmind_license_key ELSE excluded.maxmind_license_key END,
		   check_interval_seconds = excluded.check_interval_seconds,
		   pull_interval_hours = excluded.pull_interval_hours,
		   updated_at = excluded.updated_at`,
		enabled, s.MaxMindAccountID, s.MaxMindLicenseKey, s.CheckIntervalSeconds, s.PullIntervalHours,
	)
	if err != nil {
		return fmt.Errorf("geoip: save settings: %w", err)
	}
	return nil
}

// parseSettingsTime tolerates both SQLite's raw DATETIME string form
// and RFC3339 — same defensive multi-layout parse already used
// elsewhere in this codebase (e.g. ipguard's own parseTime) for a
// column whose exact returned format depends on how it was written.
func parseSettingsTime(s string) time.Time {
	for _, layout := range []string{"2006-01-02 15:04:05", time.RFC3339, time.RFC3339Nano} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}
