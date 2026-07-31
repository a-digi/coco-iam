// Package security is coco-iam's thin wrapper around
// github.com/a-digi/coco-sec — everything that stayed here after the
// extraction (see plan/coco-sec-extraction/plan.md): the admin-console
// HTTP handlers under attackbans/, geoip/, loginbans/, ipsearch/, and
// this one-shot legacy-table migration.
package security

import (
	"database/sql"
	"fmt"

	"github.com/a-digi/coco-logger/logger"
	"github.com/a-digi/coco-orm/orm"
)

// MigrateLegacySecurityTablesIfNeeded copies rows from this app's old,
// pre-extraction table names into the new coco-sec-owned ones the
// first time this runs against a database that still has the old
// tables. Safe to call on every boot: each copy is guarded by "new
// table already has rows" so it only ever runs once per table, and a
// database that never had the old tables (a fresh install) is a
// no-op throughout. Old tables are never dropped.
//
// ip_attacks/ip_attack_targets/db_meta are deliberately absent here —
// those tables kept their names across the extraction (they already
// lived in their own self-contained database, so there was never a
// naming-collision risk requiring a rename), so any existing rows are
// already compatible with coco-sec's InstallAttacks schema with no
// copy needed.
func MigrateLegacySecurityTablesIfNeeded(dbm *orm.DatabaseManager, log logger.Logger) error {
	db := dbm.Connector.DB

	if err := copyTableIfEmpty(db, log,
		"ip_bans", "security_ip_bans",
		"ip, tier, reason, banned_at, expires_at, hit_count, created_by",
	); err != nil {
		return fmt.Errorf("security: migrate ip_bans: %w", err)
	}
	if err := copyTableIfEmpty(db, log,
		"ip_allowlist", "security_ip_allowlist",
		"ip, note, created_at, created_by",
	); err != nil {
		return fmt.Errorf("security: migrate ip_allowlist: %w", err)
	}
	if err := copyTableIfEmpty(db, log,
		"attack_ban_rules", "security_attack_ban_rules",
		"id, enabled, threshold, window_seconds, ban_seconds, updated_at",
	); err != nil {
		return fmt.Errorf("security: migrate attack_ban_rules: %w", err)
	}
	if err := copyTableIfEmpty(db, log,
		"login_ban_rules", "security_login_ban_rules",
		"id, admin_enabled, admin_threshold, admin_window_seconds, admin_ban_seconds, "+
			"application_enabled, application_threshold, application_window_seconds, application_ban_seconds, updated_at",
	); err != nil {
		return fmt.Errorf("security: migrate login_ban_rules: %w", err)
	}
	if err := copyTableIfEmpty(db, log,
		"geoip_settings", "security_geoip_settings",
		"id, enabled, maxmind_account_id, maxmind_license_key, check_interval_seconds, pull_interval_hours, updated_at",
	); err != nil {
		return fmt.Errorf("security: migrate geoip_settings: %w", err)
	}
	return nil
}

// copyTableIfEmpty copies every row of oldTable into newTable
// (identical column list on both sides) when: oldTable exists, and
// newTable is currently empty. Both conditions must hold, so this is
// safe to call unconditionally on every boot.
func copyTableIfEmpty(db *sql.DB, log logger.Logger, oldTable, newTable, columns string) error {
	exists, err := tableExists(db, oldTable)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}

	var newCount int
	if err := db.QueryRow(fmt.Sprintf(`SELECT COUNT(1) FROM %s`, newTable)).Scan(&newCount); err != nil {
		return fmt.Errorf("count %s: %w", newTable, err)
	}
	if newCount > 0 {
		return nil
	}

	var oldCount int
	if err := db.QueryRow(fmt.Sprintf(`SELECT COUNT(1) FROM %s`, oldTable)).Scan(&oldCount); err != nil {
		return fmt.Errorf("count %s: %w", oldTable, err)
	}
	if oldCount == 0 {
		return nil
	}

	res, err := db.Exec(fmt.Sprintf(`INSERT INTO %s (%s) SELECT %s FROM %s`, newTable, columns, columns, oldTable))
	if err != nil {
		return fmt.Errorf("copy %s -> %s: %w", oldTable, newTable, err)
	}
	n, _ := res.RowsAffected()
	if log != nil {
		log.Info("security migration: copied %d row(s) from %s to %s", n, oldTable, newTable)
	}
	return nil
}

func tableExists(db *sql.DB, name string) (bool, error) {
	var n int
	err := db.QueryRow(`SELECT COUNT(1) FROM sqlite_master WHERE type='table' AND name=?`, name).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("check table %s exists: %w", name, err)
	}
	return n > 0, nil
}
