package store

import (
	"fmt"

	"github.com/a-digi/coco-orm/orm"
)

// Install creates the mail_outbound table + indexes on the supplied
// DatabaseManager. Safe to call on every boot — all statements are
// `IF NOT EXISTS`.
func Install(dbm *orm.DatabaseManager) error {
	if dbm == nil || dbm.Connector == nil || dbm.Connector.DB == nil {
		return fmt.Errorf("mail store: database manager not initialised")
	}
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS mail_outbound (
			id TEXT PRIMARY KEY,
			template TEXT NOT NULL DEFAULT '',
			subject TEXT NOT NULL,
			from_email TEXT NOT NULL,
			from_name TEXT NOT NULL DEFAULT '',
			to_json TEXT NOT NULL,
			cc_json TEXT NOT NULL DEFAULT '[]',
			bcc_json TEXT NOT NULL DEFAULT '[]',
			status TEXT NOT NULL DEFAULT 'queued',
			attempts INTEGER NOT NULL DEFAULT 0,
			max_attempts INTEGER NOT NULL DEFAULT 5,
			last_error TEXT NOT NULL DEFAULT '',
			next_attempt_at TEXT NOT NULL DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			sent_at TEXT NOT NULL DEFAULT ''
		);`,
		`CREATE INDEX IF NOT EXISTS mail_outbound_status_idx ON mail_outbound(status);`,
		`CREATE INDEX IF NOT EXISTS mail_outbound_created_at_idx ON mail_outbound(created_at);`,
		`CREATE INDEX IF NOT EXISTS mail_outbound_to_idx ON mail_outbound(to_json);`,

		`CREATE TABLE IF NOT EXISTS mail_templates (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			description TEXT NOT NULL DEFAULT '',
			subject TEXT NOT NULL DEFAULT '',
			text_body TEXT NOT NULL DEFAULT '',
			html_body TEXT NOT NULL DEFAULT '',
			is_active BOOLEAN NOT NULL DEFAULT TRUE,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE UNIQUE INDEX IF NOT EXISTS mail_templates_name_uindex ON mail_templates(name);`,

		`CREATE TABLE IF NOT EXISTS mail_settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL DEFAULT '',
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,

		`CREATE TABLE IF NOT EXISTS mail_smtp_accounts (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			host TEXT NOT NULL,
			port INTEGER NOT NULL DEFAULT 587,
			username TEXT NOT NULL DEFAULT '',
			password TEXT NOT NULL DEFAULT '',
			from_name TEXT NOT NULL DEFAULT '',
			from_email TEXT NOT NULL DEFAULT '',
			use_tls BOOLEAN NOT NULL DEFAULT FALSE,
			is_active BOOLEAN NOT NULL DEFAULT FALSE,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE UNIQUE INDEX IF NOT EXISTS mail_smtp_accounts_name_uindex ON mail_smtp_accounts(name);`,
		`CREATE UNIQUE INDEX IF NOT EXISTS mail_smtp_accounts_active_uindex ON mail_smtp_accounts(is_active) WHERE is_active = TRUE;`,
	}
	for _, s := range stmts {
		if _, err := dbm.Connector.DB.Exec(s); err != nil {
			return fmt.Errorf("mail store: install %q: %w", s, err)
		}
	}
	return nil
}
