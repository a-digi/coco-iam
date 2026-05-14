package datamigration

import (
	"database/sql"
	"fmt"

	"github.com/a-digi/coco-iam/src/organizations/users/dbregistry"
	"github.com/a-digi/coco-iam/src/orgrouter"
	"github.com/a-digi/coco-logger/logger"
)

// MigrateActivationTablesToOrgDBs copies user-type rows from the global
// user_activations table into each organisation's per-org users.db, then
// drops the global table.
//
// Must be called after orgUserDBRegistry.SweepExisting() so per-org table
// schemas (migration 25_user_activations.sql) already exist. Idempotent:
// returns nil immediately when user_activations is no longer in the global DB.
func MigrateActivationTablesToOrgDBs(
	mainDB *sql.DB,
	reg *dbregistry.OrgUserDBRegistry,
	log logger.Logger,
) error {
	if !tableExists(mainDB, "user_activations") {
		return nil
	}

	orgIDs := reg.KnownOrgIDs()

	allOK := true
	for _, orgID := range orgIDs {
		orgDB, oErr := orgrouter.ForOrg(reg, orgID)
		if oErr != nil {
			log.Warning("activation migration: open org db %s: %v", orgID, oErr)
			allOK = false
			continue
		}
		if cErr := copyOrgActivations(mainDB, orgDB, orgID); cErr != nil {
			log.Warning("activation migration: org %s: %v", orgID, cErr)
			allOK = false
		}
	}

	if !allOK {
		return fmt.Errorf("activation migration: some orgs failed -- global table preserved for retry")
	}

	if _, err := mainDB.Exec(`DROP TABLE IF EXISTS user_activations`); err != nil {
		return fmt.Errorf("activation migration: drop global table: %w", err)
	}
	log.Info("activation migration: global user_activations table dropped")
	return nil
}

func copyOrgActivations(mainDB, orgDB *sql.DB, orgID string) error {
	return copyRows(mainDB, orgDB,
		`SELECT id, user_id, token_hash, temp_password_hash, expires_at,
		        consumed_at, created_at,
		        redirect_organization_slug, redirect_workspace_slug, redirect_application_client_id
		 FROM user_activations
		 WHERE user_type = 'user' AND org_id = ?`,
		`INSERT OR IGNORE INTO user_activations
		 (id, user_id, token_hash, temp_password_hash, expires_at, consumed_at, created_at,
		  redirect_organization_slug, redirect_workspace_slug, redirect_application_client_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		orgID,
	)
}
