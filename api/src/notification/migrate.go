package notification

import (
	"database/sql"
	"fmt"

	"github.com/a-digi/coco-logger/logger"
	"github.com/a-digi/coco-orm/orm"
)

// MigrateLegacyMailTablesIfNeeded copies rows from this app's old,
// now-retired api/src/mail schema (mail_smtp_accounts, mail_templates,
// mail_settings) into the new coco-notification-owned tables
// (mailer_smtp_accounts, notification_templates, notification_settings)
// the first time this runs against a database that still has the old
// tables. Safe to call on every boot: each copy is guarded by "new
// table already has rows" so it only ever runs once per table, and a
// database that never had the old tables (a fresh install) is a
// no-op throughout.
//
// mail_outbound (historical send log) is deliberately NOT migrated —
// it's operational history, not live configuration, and every row in
// it is already terminal (sent/failed/dead_lettered) by the time this
// runs. The old table and its rows are left untouched (never
// dropped) so they remain manually inspectable via sqlite3 after the
// cutover.
func MigrateLegacyMailTablesIfNeeded(dbm *orm.DatabaseManager, log logger.Logger) error {
	db := dbm.Connector.DB

	if err := copyTableIfEmpty(db, log,
		"mail_smtp_accounts", "mailer_smtp_accounts",
		"id, name, host, port, username, password, from_name, from_email, use_tls, is_active, created_at, updated_at",
	); err != nil {
		return fmt.Errorf("notification: migrate mail_smtp_accounts: %w", err)
	}
	if err := copyTableIfEmpty(db, log,
		"mail_templates", "notification_templates",
		"id, name, description, subject, text_body, html_body, is_active, created_at, updated_at",
	); err != nil {
		return fmt.Errorf("notification: migrate mail_templates: %w", err)
	}
	if err := copyTableIfEmpty(db, log,
		"mail_settings", "notification_settings",
		"key, value, updated_at",
	); err != nil {
		return fmt.Errorf("notification: migrate mail_settings: %w", err)
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
		log.Info("notification migration: copied %d row(s) from %s to %s", n, oldTable, newTable)
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
