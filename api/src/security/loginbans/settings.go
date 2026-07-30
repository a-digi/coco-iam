// Package loginbans owns the admin-editable failed-login ban-rule
// settings — one global threshold for admin-console logins, one for
// application end-user logins across every application (not
// per-application). Stored in the main DB's login_ban_rules singleton
// row, same pattern as geoip.Settings/geoip_settings. See
// plan/login-ban-rules/plan.md.
package loginbans

import (
	"database/sql"
	"fmt"
	"time"
)

// DomainRule is one domain's (admin or application) failed-login ban
// rule: ban an IP once it has Threshold failed logins within
// WindowSeconds, for BanSeconds.
type DomainRule struct {
	Enabled       bool
	Threshold     int
	WindowSeconds int
	BanSeconds    int
}

// Settings is the full admin-editable ban-rule configuration.
type Settings struct {
	Admin       DomainRule
	Application DomainRule
	UpdatedAt   time.Time

	// loaded is true only when this Settings came from an actual
	// stored row — unexported, purely an internal signal, mirrors
	// geoip.Settings' own field of the same name.
	loaded bool
}

// DefaultSettings returns both domains disabled with the same
// threshold/window/ban defaults the migration itself ships — used
// when no row has ever been saved (a fresh install nobody has
// configured yet).
func DefaultSettings() Settings {
	return Settings{
		Admin:       DomainRule{Enabled: false, Threshold: 5, WindowSeconds: 600, BanSeconds: 3600},
		Application: DomainRule{Enabled: false, Threshold: 5, WindowSeconds: 600, BanSeconds: 3600},
	}
}

// SettingsQueryRepo reads the login_ban_rules singleton row from the
// main database.
type SettingsQueryRepo struct {
	db *sql.DB
}

func NewSettingsQueryRepo(db *sql.DB) *SettingsQueryRepo {
	return &SettingsQueryRepo{db: db}
}

// LoadSettings returns the current settings, or DefaultSettings()
// (never an error) if no row has ever been saved — a fresh install
// with nobody having visited the settings page yet is an expected
// state, not a failure. Mirrors geoip.SettingsQueryRepo.LoadSettings.
func (r *SettingsQueryRepo) LoadSettings() (Settings, error) {
	var s Settings
	var adminEnabled, appEnabled int
	var updatedAt sql.NullString
	err := r.db.QueryRow(
		`SELECT admin_enabled, admin_threshold, admin_window_seconds, admin_ban_seconds,
		        application_enabled, application_threshold, application_window_seconds, application_ban_seconds,
		        updated_at
		 FROM login_ban_rules WHERE id = 1`,
	).Scan(
		&adminEnabled, &s.Admin.Threshold, &s.Admin.WindowSeconds, &s.Admin.BanSeconds,
		&appEnabled, &s.Application.Threshold, &s.Application.WindowSeconds, &s.Application.BanSeconds,
		&updatedAt,
	)
	if err == sql.ErrNoRows {
		return DefaultSettings(), nil
	}
	if err != nil {
		return Settings{}, fmt.Errorf("loginbans: load settings: %w", err)
	}
	s.Admin.Enabled = adminEnabled != 0
	s.Application.Enabled = appEnabled != 0
	s.loaded = true
	if updatedAt.Valid {
		s.UpdatedAt = parseSettingsTime(updatedAt.String)
	}
	return s, nil
}

// SettingsPersistentRepo writes the login_ban_rules singleton row.
type SettingsPersistentRepo struct {
	db *sql.DB
}

func NewSettingsPersistentRepo(db *sql.DB) *SettingsPersistentRepo {
	return &SettingsPersistentRepo{db: db}
}

// SaveSettings upserts the singleton row.
func (r *SettingsPersistentRepo) SaveSettings(s Settings) error {
	adminEnabled, appEnabled := 0, 0
	if s.Admin.Enabled {
		adminEnabled = 1
	}
	if s.Application.Enabled {
		appEnabled = 1
	}
	_, err := r.db.Exec(
		`INSERT INTO login_ban_rules
		   (id, admin_enabled, admin_threshold, admin_window_seconds, admin_ban_seconds,
		    application_enabled, application_threshold, application_window_seconds, application_ban_seconds, updated_at)
		 VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(id) DO UPDATE SET
		   admin_enabled = excluded.admin_enabled,
		   admin_threshold = excluded.admin_threshold,
		   admin_window_seconds = excluded.admin_window_seconds,
		   admin_ban_seconds = excluded.admin_ban_seconds,
		   application_enabled = excluded.application_enabled,
		   application_threshold = excluded.application_threshold,
		   application_window_seconds = excluded.application_window_seconds,
		   application_ban_seconds = excluded.application_ban_seconds,
		   updated_at = excluded.updated_at`,
		adminEnabled, s.Admin.Threshold, s.Admin.WindowSeconds, s.Admin.BanSeconds,
		appEnabled, s.Application.Threshold, s.Application.WindowSeconds, s.Application.BanSeconds,
	)
	if err != nil {
		return fmt.Errorf("loginbans: save settings: %w", err)
	}
	return nil
}

// parseSettingsTime tolerates both SQLite's raw DATETIME string form
// and RFC3339 — same defensive multi-layout parse used elsewhere in
// this codebase (e.g. geoip.parseSettingsTime, ipguard's parseTime).
func parseSettingsTime(s string) time.Time {
	for _, layout := range []string{"2006-01-02 15:04:05", time.RFC3339, time.RFC3339Nano} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}
