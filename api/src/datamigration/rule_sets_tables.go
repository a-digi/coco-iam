package datamigration

import (
	"database/sql"
	"fmt"

	"github.com/a-digi/coco-iam/src/organizations/users/dbregistry"
	"github.com/a-digi/coco-iam/src/orgrouter"
	"github.com/a-digi/coco-logger/logger"
)

// MigrateRuleSetsTablesToOrgDBs copies org-scoped rows from the global
// user_rule_sets table into each org's per-org users.db, copies the admin
// row to admin_user_rule_sets, then drops the global table.
//
// Must be called after orgUserDBRegistry.SweepExisting() so per-org table
// schemas (migration 28_user_rule_sets.sql) already exist. Idempotent:
// returns nil immediately when user_rule_sets is no longer in the global DB.
func MigrateRuleSetsTablesToOrgDBs(
	mainDB *sql.DB,
	reg *dbregistry.OrgUserDBRegistry,
	log logger.Logger,
) error {
	if !tableExists(mainDB, "user_rule_sets") {
		return nil
	}

	// Copy the admin row to admin_user_rule_sets.
	var adminJSON string
	if err := mainDB.QueryRow(
		`SELECT rules_json FROM user_rule_sets WHERE scope = 'admin' AND owner_id = 'admin' LIMIT 1`,
	).Scan(&adminJSON); err == nil {
		if _, err := mainDB.Exec(
			`INSERT OR IGNORE INTO admin_user_rule_sets (id, rules_json, updated_at)
			 VALUES ('admin', ?, CURRENT_TIMESTAMP)`,
			adminJSON,
		); err != nil {
			log.Warning("rule sets migration: insert admin row: %v", err)
		}
	}

	// Copy org-scoped rows.
	rows, err := mainDB.Query(
		`SELECT owner_id, rules_json FROM user_rule_sets WHERE scope = 'organization'`,
	)
	if err != nil {
		return fmt.Errorf("rule sets migration: query org rows: %w", err)
	}
	defer rows.Close()

	type orgRow struct {
		orgID     string
		rulesJSON string
	}

	var pending []orgRow
	for rows.Next() {
		var r orgRow
		if err := rows.Scan(&r.orgID, &r.rulesJSON); err != nil {
			return fmt.Errorf("rule sets migration: scan row: %w", err)
		}
		pending = append(pending, r)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("rule sets migration: iterate rows: %w", err)
	}

	allOK := true
	for _, r := range pending {
		orgDB, err := orgrouter.ForOrg(reg, r.orgID)
		if err != nil {
			log.Warning("rule sets migration: open org db for %s: %v — skipping", r.orgID, err)
			allOK = false
			continue
		}
		if _, err := orgDB.Exec(
			`INSERT OR IGNORE INTO user_rule_sets (id, rules_json, updated_at)
			 VALUES ('default', ?, CURRENT_TIMESTAMP)`,
			r.rulesJSON,
		); err != nil {
			log.Warning("rule sets migration: insert for org %s: %v — skipping", r.orgID, err)
			allOK = false
		}
	}

	if !allOK {
		return fmt.Errorf("rule sets migration: some rows failed -- global table preserved for retry")
	}

	if _, err := mainDB.Exec(`DROP TABLE IF EXISTS user_rule_sets`); err != nil {
		return fmt.Errorf("rule sets migration: drop global table: %w", err)
	}
	log.Info("rule sets migration: global user_rule_sets table dropped")
	return nil
}
