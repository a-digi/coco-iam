package datamigration

import (
	"database/sql"
	"fmt"

	"github.com/a-digi/coco-iam/src/organizations/users/dbregistry"
	"github.com/a-digi/coco-iam/src/orgrouter"
	"github.com/a-digi/coco-logger/logger"
)

// MigrateRecoveryTablesToOrgDBs copies user-type rows from the global
// password_recoveries table into each org's per-org users.db, then drops
// the global table.
//
// Must be called after orgUserDBRegistry.SweepExisting() so per-org table
// schemas (migration 27_password_recoveries.sql) already exist. Idempotent:
// returns nil immediately when password_recoveries is no longer in the
// global DB.
func MigrateRecoveryTablesToOrgDBs(
	mainDB *sql.DB,
	reg *dbregistry.OrgUserDBRegistry,
	log logger.Logger,
) error {
	if !tableExists(mainDB, "password_recoveries") {
		return nil
	}

	rows, err := mainDB.Query(
		`SELECT id, user_id, token_hash, expires_at, consumed_at, created_at
		 FROM password_recoveries WHERE user_type = 'user'`,
	)
	if err != nil {
		return fmt.Errorf("recovery migration: query user rows: %w", err)
	}
	defer rows.Close()

	type recoveryRow struct {
		id, userID, tokenHash, expiresAt string
		consumedAt, createdAt            sql.NullString
	}

	var pending []recoveryRow
	for rows.Next() {
		var r recoveryRow
		if err := rows.Scan(&r.id, &r.userID, &r.tokenHash, &r.expiresAt, &r.consumedAt, &r.createdAt); err != nil {
			return fmt.Errorf("recovery migration: scan row: %w", err)
		}
		pending = append(pending, r)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("recovery migration: iterate rows: %w", err)
	}

	allOK := true
	for _, r := range pending {
		orgDB, _, err := orgrouter.OrgDBFor(reg, r.userID)
		if err != nil {
			log.Warning("recovery migration: find org for user %s: %v — skipping", r.userID, err)
			allOK = false
			continue
		}
		args := []interface{}{r.id, r.userID, r.tokenHash, r.expiresAt}
		var consumedVal interface{}
		if r.consumedAt.Valid {
			consumedVal = r.consumedAt.String
		}
		args = append(args, consumedVal)
		var createdVal interface{}
		if r.createdAt.Valid {
			createdVal = r.createdAt.String
		}
		args = append(args, createdVal)
		if _, err := orgDB.Exec(
			`INSERT OR IGNORE INTO password_recoveries
			 (id, user_id, token_hash, expires_at, consumed_at, created_at)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			args...,
		); err != nil {
			log.Warning("recovery migration: insert for user %s: %v — skipping", r.userID, err)
			allOK = false
		}
	}

	if !allOK {
		return fmt.Errorf("recovery migration: some rows failed -- global table preserved for retry")
	}

	if _, err := mainDB.Exec(`DROP TABLE IF EXISTS password_recoveries`); err != nil {
		return fmt.Errorf("recovery migration: drop global table: %w", err)
	}
	log.Info("recovery migration: global password_recoveries table dropped")
	return nil
}
