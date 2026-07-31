// Package attackbans owns the admin-editable ban-rule settings for
// high-volume scan/probe traffic — a single global threshold (unlike
// loginbans' admin/application split, scan traffic against
// nonexistent routes isn't scoped to a login domain). Stored in the
// main DB's attack_ban_rules singleton row, same pattern as
// loginbans.Settings/geoip.Settings. See plan/attack-ban-rules/plan.md.
package attackbans

import (
	"database/sql"
	"fmt"
	"time"
)

// Settings is the admin-editable ban rule: ban an IP once it has
// Threshold hits against nonexistent routes within WindowSeconds, for
// BanSeconds.
type Settings struct {
	Enabled       bool
	Threshold     int
	WindowSeconds int
	BanSeconds    int
	UpdatedAt     time.Time

	// loaded is true only when this Settings came from an actual
	// stored row — unexported, purely an internal signal, mirrors
	// loginbans.Settings' own field of the same name.
	loaded bool
}

// DefaultSettings returns disabled with the same threshold/window/ban
// defaults the migration itself ships — used when no row has ever
// been saved (a fresh install nobody has configured yet).
func DefaultSettings() Settings {
	return Settings{Enabled: false, Threshold: 50, WindowSeconds: 60, BanSeconds: 3600}
}

// SettingsQueryRepo reads the attack_ban_rules singleton row from the
// main database.
type SettingsQueryRepo struct {
	db *sql.DB
}

func NewSettingsQueryRepo(db *sql.DB) *SettingsQueryRepo {
	return &SettingsQueryRepo{db: db}
}

// LoadSettings returns the current settings, or DefaultSettings()
// (never an error) if no row has ever been saved. Mirrors
// loginbans.SettingsQueryRepo.LoadSettings.
func (r *SettingsQueryRepo) LoadSettings() (Settings, error) {
	var s Settings
	var enabled int
	var updatedAt sql.NullString
	err := r.db.QueryRow(
		`SELECT enabled, threshold, window_seconds, ban_seconds, updated_at
		 FROM attack_ban_rules WHERE id = 1`,
	).Scan(&enabled, &s.Threshold, &s.WindowSeconds, &s.BanSeconds, &updatedAt)
	if err == sql.ErrNoRows {
		return DefaultSettings(), nil
	}
	if err != nil {
		return Settings{}, fmt.Errorf("attackbans: load settings: %w", err)
	}
	s.Enabled = enabled != 0
	s.loaded = true
	if updatedAt.Valid {
		s.UpdatedAt = parseSettingsTime(updatedAt.String)
	}
	return s, nil
}

// SettingsPersistentRepo writes the attack_ban_rules singleton row.
type SettingsPersistentRepo struct {
	db *sql.DB
}

func NewSettingsPersistentRepo(db *sql.DB) *SettingsPersistentRepo {
	return &SettingsPersistentRepo{db: db}
}

// SaveSettings upserts the singleton row.
func (r *SettingsPersistentRepo) SaveSettings(s Settings) error {
	enabled := 0
	if s.Enabled {
		enabled = 1
	}
	_, err := r.db.Exec(
		`INSERT INTO attack_ban_rules (id, enabled, threshold, window_seconds, ban_seconds, updated_at)
		 VALUES (1, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(id) DO UPDATE SET
		   enabled = excluded.enabled,
		   threshold = excluded.threshold,
		   window_seconds = excluded.window_seconds,
		   ban_seconds = excluded.ban_seconds,
		   updated_at = excluded.updated_at`,
		enabled, s.Threshold, s.WindowSeconds, s.BanSeconds,
	)
	if err != nil {
		return fmt.Errorf("attackbans: save settings: %w", err)
	}
	return nil
}

// parseSettingsTime tolerates both SQLite's raw DATETIME string form
// and RFC3339 — same defensive multi-layout parse used elsewhere in
// this codebase (e.g. loginbans.parseSettingsTime).
func parseSettingsTime(s string) time.Time {
	for _, layout := range []string{"2006-01-02 15:04:05", time.RFC3339, time.RFC3339Nano} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}
