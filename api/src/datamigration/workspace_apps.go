// Package datamigration holds one-time boot migrations that cannot be
// expressed as pure SQL because they move data between the global DB and
// the per-organisation user databases.
package datamigration

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/a-digi/coco-iam/src/organizations/users/dbregistry"
	"github.com/a-digi/coco-iam/src/orgrouter"
	"github.com/a-digi/coco-logger/logger"
)

// MigrateWorkspaceAndAppsToOrgDBs copies workspace, applications, and their
// seven child tables from the global users.db into each organisation's
// per-org users.db, then drops the global copies.
//
// Must be called after orgUserDBRegistry.SweepExisting() so per-org table
// schemas (migrations 10-18) already exist. Idempotent: returns nil
// immediately when the workspace table is no longer in the global DB.
func MigrateWorkspaceAndAppsToOrgDBs(
	mainDB *sql.DB,
	reg *dbregistry.OrgUserDBRegistry,
	log logger.Logger,
) error {
	if !tableExists(mainDB, "workspace") {
		return nil
	}

	orgIDs, err := distinctOrgIDs(mainDB)
	if err != nil {
		return fmt.Errorf("workspace migration: list orgs: %w", err)
	}

	allOK := true
	for _, orgID := range orgIDs {
		orgDB, oErr := orgrouter.ForOrg(reg, orgID)
		if oErr != nil {
			log.Warning("workspace migration: open org db %s: %v", orgID, oErr)
			allOK = false
			continue
		}
		if cErr := copyOrgData(mainDB, orgDB, orgID); cErr != nil {
			log.Warning("workspace migration: org %s: %v", orgID, cErr)
			allOK = false
		}
	}

	if !allOK {
		return fmt.Errorf("workspace migration: some orgs failed -- global tables preserved for retry")
	}

	if err := dropGlobalTables(mainDB); err != nil {
		return fmt.Errorf("workspace migration: drop global tables: %w", err)
	}
	log.Info("workspace/apps migration: global tables dropped")
	return nil
}

func tableExists(db *sql.DB, name string) bool {
	var n int
	_ = db.QueryRow(
		`SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?`, name,
	).Scan(&n)
	return n > 0
}

func distinctOrgIDs(mainDB *sql.DB) ([]string, error) {
	rows, err := mainDB.Query(
		`SELECT DISTINCT organization_id FROM workspace WHERE organization_id != ''`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func copyOrgData(mainDB, orgDB *sql.DB, orgID string) error {
	if err := copyWorkspaces(mainDB, orgDB, orgID); err != nil {
		return fmt.Errorf("workspaces: %w", err)
	}
	appIDs, err := copyApplications(mainDB, orgDB, orgID)
	if err != nil {
		return fmt.Errorf("applications: %w", err)
	}
	if len(appIDs) == 0 {
		return nil
	}
	if err := copyApplicationKeys(mainDB, orgDB, appIDs); err != nil {
		return fmt.Errorf("application_keys: %w", err)
	}
	if err := copyLoginSettings(mainDB, orgDB, appIDs); err != nil {
		return fmt.Errorf("application_login_settings: %w", err)
	}
	if err := copyLoginAssets(mainDB, orgDB, appIDs); err != nil {
		return fmt.Errorf("application_login_assets: %w", err)
	}
	if err := copyLoginColumns(mainDB, orgDB, appIDs); err != nil {
		return fmt.Errorf("application_login_columns: %w", err)
	}
	if err := copyOAuthClients(mainDB, orgDB, appIDs); err != nil {
		return fmt.Errorf("application_oauth_clients: %w", err)
	}
	if err := copyOAuthProviders(mainDB, orgDB, appIDs); err != nil {
		return fmt.Errorf("application_oauth_providers: %w", err)
	}
	if err := copyRecoveryTemplates(mainDB, orgDB, appIDs); err != nil {
		return fmt.Errorf("application_recovery_templates: %w", err)
	}
	return nil
}

// copyRows reads all rows via selectSQL on src and inserts each into dst via
// insertSQL. The SELECT column list and VALUES placeholder count must match.
func copyRows(src, dst *sql.DB, selectSQL, insertSQL string, args ...interface{}) error {
	rows, err := src.Query(selectSQL, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return err
	}
	for rows.Next() {
		vals := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return err
		}
		if _, err := dst.Exec(insertSQL, vals...); err != nil {
			return err
		}
	}
	return rows.Err()
}

func inPlaceholders(n int) string {
	if n == 0 {
		return ""
	}
	return strings.Repeat("?,", n-1) + "?"
}

func toAnySlice(ids []string) []interface{} {
	out := make([]interface{}, len(ids))
	for i, id := range ids {
		out[i] = id
	}
	return out
}

func copyWorkspaces(mainDB, orgDB *sql.DB, orgID string) error {
	return copyRows(mainDB, orgDB,
		`SELECT id, title, description, organization_id, workspace_id, created_at, is_active
		 FROM workspace WHERE organization_id = ?`,
		`INSERT OR IGNORE INTO workspace
		 (id, title, description, organization_id, workspace_id, created_at, is_active)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		orgID,
	)
}

func copyApplications(mainDB, orgDB *sql.DB, orgID string) ([]string, error) {
	var appIDs []string
	{
		rows, err := mainDB.Query(
			`SELECT a.id FROM applications a
			 JOIN workspace w ON w.id = a.workspace_id
			 WHERE w.organization_id = ?`, orgID,
		)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id string
			if sErr := rows.Scan(&id); sErr != nil {
				rows.Close()
				return nil, sErr
			}
			appIDs = append(appIDs, id)
		}
		if rErr := rows.Err(); rErr != nil {
			rows.Close()
			return nil, rErr
		}
		rows.Close()
	}
	if len(appIDs) == 0 {
		return nil, nil
	}
	ph := inPlaceholders(len(appIDs))
	err := copyRows(mainDB, orgDB,
		`SELECT id, workspace_id, client_id, title, description, created_at, is_active,
		        allow_recovery, allow_registration, registration_type, allow_password_login
		 FROM applications WHERE id IN (`+ph+`)`,
		`INSERT OR IGNORE INTO applications
		 (id, workspace_id, client_id, title, description, created_at, is_active,
		  allow_recovery, allow_registration, registration_type, allow_password_login)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		toAnySlice(appIDs)...,
	)
	return appIDs, err
}

func copyApplicationKeys(mainDB, orgDB *sql.DB, appIDs []string) error {
	ph := inPlaceholders(len(appIDs))
	return copyRows(mainDB, orgDB,
		`SELECT id, application_id, status, created_at, activated_at, deactivated_at, expires_at
		 FROM application_keys WHERE application_id IN (`+ph+`)`,
		`INSERT OR IGNORE INTO application_keys
		 (id, application_id, status, created_at, activated_at, deactivated_at, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		toAnySlice(appIDs)...,
	)
}

func copyLoginSettings(mainDB, orgDB *sql.DB, appIDs []string) error {
	ph := inPlaceholders(len(appIDs))
	return copyRows(mainDB, orgDB,
		`SELECT application_id, redirect_url, redirect_method, redirect_secret,
		        custom_headers, template_kind, background_color, background_asset_id,
		        show_logo, page_title, brand_text, background_gradient_from,
		        background_gradient_to, background_gradient_angle, rich_text_defaults,
		        oauth_client_id, updated_at
		 FROM application_login_settings WHERE application_id IN (`+ph+`)`,
		`INSERT OR IGNORE INTO application_login_settings
		 (application_id, redirect_url, redirect_method, redirect_secret,
		  custom_headers, template_kind, background_color, background_asset_id,
		  show_logo, page_title, brand_text, background_gradient_from,
		  background_gradient_to, background_gradient_angle, rich_text_defaults,
		  oauth_client_id, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		toAnySlice(appIDs)...,
	)
}

func copyLoginAssets(mainDB, orgDB *sql.DB, appIDs []string) error {
	ph := inPlaceholders(len(appIDs))
	return copyRows(mainDB, orgDB,
		`SELECT id, application_id, file_path, mime_type, size_bytes, kind, created_at
		 FROM application_login_assets WHERE application_id IN (`+ph+`)`,
		`INSERT OR IGNORE INTO application_login_assets
		 (id, application_id, file_path, mime_type, size_bytes, kind, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		toAnySlice(appIDs)...,
	)
}

func copyLoginColumns(mainDB, orgDB *sql.DB, appIDs []string) error {
	ph := inPlaceholders(len(appIDs))
	return copyRows(mainDB, orgDB,
		`SELECT application_id, column_index, background_color, background_asset_id,
		        background_gradient_from, background_gradient_to, background_gradient_angle,
		        text_color_override, text_block_title, text_contents, updated_at
		 FROM application_login_columns WHERE application_id IN (`+ph+`)`,
		`INSERT OR IGNORE INTO application_login_columns
		 (application_id, column_index, background_color, background_asset_id,
		  background_gradient_from, background_gradient_to, background_gradient_angle,
		  text_color_override, text_block_title, text_contents, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		toAnySlice(appIDs)...,
	)
}

func copyOAuthClients(mainDB, orgDB *sql.DB, appIDs []string) error {
	ph := inPlaceholders(len(appIDs))
	return copyRows(mainDB, orgDB,
		`SELECT id, application_id, client_id, client_secret_hash, client_type,
		        display_name, redirect_uris, allowed_scopes, require_consent,
		        access_token_ttl, refresh_token_ttl, is_active, created_at, updated_at
		 FROM application_oauth_clients WHERE application_id IN (`+ph+`)`,
		`INSERT OR IGNORE INTO application_oauth_clients
		 (id, application_id, client_id, client_secret_hash, client_type,
		  display_name, redirect_uris, allowed_scopes, require_consent,
		  access_token_ttl, refresh_token_ttl, is_active, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		toAnySlice(appIDs)...,
	)
}

func copyOAuthProviders(mainDB, orgDB *sql.DB, appIDs []string) error {
	ph := inPlaceholders(len(appIDs))
	return copyRows(mainDB, orgDB,
		`SELECT id, application_id, provider, client_id, client_secret_enc,
		        discovery_url, authorize_url, token_url, userinfo_url, scopes,
		        allow_login, allow_registration, is_active, created_at, updated_at
		 FROM application_oauth_providers WHERE application_id IN (`+ph+`)`,
		`INSERT OR IGNORE INTO application_oauth_providers
		 (id, application_id, provider, client_id, client_secret_enc,
		  discovery_url, authorize_url, token_url, userinfo_url, scopes,
		  allow_login, allow_registration, is_active, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		toAnySlice(appIDs)...,
	)
}

func copyRecoveryTemplates(mainDB, orgDB *sql.DB, appIDs []string) error {
	ph := inPlaceholders(len(appIDs))
	return copyRows(mainDB, orgDB,
		`SELECT application_id, request_body_html, reset_body_html, updated_at
		 FROM application_recovery_templates WHERE application_id IN (`+ph+`)`,
		`INSERT OR IGNORE INTO application_recovery_templates
		 (application_id, request_body_html, reset_body_html, updated_at)
		 VALUES (?, ?, ?, ?)`,
		toAnySlice(appIDs)...,
	)
}

func dropGlobalTables(mainDB *sql.DB) error {
	stmts := []string{
		`DROP TABLE IF EXISTS application_recovery_templates`,
		`DROP TABLE IF EXISTS application_oauth_providers`,
		`DROP TABLE IF EXISTS application_oauth_clients`,
		`DROP TABLE IF EXISTS application_login_columns`,
		`DROP TABLE IF EXISTS application_login_assets`,
		`DROP TABLE IF EXISTS application_login_settings`,
		`DROP TABLE IF EXISTS application_keys`,
		`DROP TABLE IF EXISTS applications`,
		`DROP TABLE IF EXISTS workspace`,
	}
	for _, stmt := range stmts {
		if _, err := mainDB.Exec(stmt); err != nil {
			return fmt.Errorf("%s: %w", stmt, err)
		}
	}
	return nil
}
