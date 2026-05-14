package datamigration

import (
	"database/sql"
	"fmt"

	oauthdbregistry "github.com/a-digi/coco-iam/src/oauthserver/dbregistry"
	"github.com/a-digi/coco-logger/logger"
)

// MigrateOAuthTablesToOrgDBs copies oauth_auth_requests,
// oauth_authorization_codes, and oauth_refresh_tokens from the global
// users.db into each organisation's per-org oauth.db, then drops the
// global copies.
//
// Must be called after orgOAuthDBRegistry.SweepExisting() so per-org
// table schemas already exist. Idempotent: returns nil immediately
// when oauth_authorization_codes is no longer in the global DB.
func MigrateOAuthTablesToOrgDBs(
	mainDB *sql.DB,
	reg *oauthdbregistry.OrgOAuthDBRegistry,
	log logger.Logger,
) error {
	if !tableExists(mainDB, "oauth_authorization_codes") {
		return nil
	}

	orgIDs := reg.KnownOrgIDs()

	allOK := true
	for _, orgID := range orgIDs {
		mgr, oErr := reg.For(orgID)
		if oErr != nil {
			log.Warning("oauth migration: open oauth db %s: %v", orgID, oErr)
			allOK = false
			continue
		}
		orgDB := mgr.Connector.DB
		if cErr := copyOAuthData(mainDB, orgDB, orgID); cErr != nil {
			log.Warning("oauth migration: org %s: %v", orgID, cErr)
			allOK = false
		}
	}

	if !allOK {
		return fmt.Errorf("oauth migration: some orgs failed -- global tables preserved for retry")
	}

	if err := dropGlobalOAuthTables(mainDB); err != nil {
		return fmt.Errorf("oauth migration: drop global tables: %w", err)
	}
	log.Info("oauth migration: global tables dropped")
	return nil
}

func copyOAuthData(mainDB, orgDB *sql.DB, orgID string) error {
	if err := copyAuthRequests(mainDB, orgDB, orgID); err != nil {
		return fmt.Errorf("oauth_auth_requests: %w", err)
	}
	if err := copyAuthorizationCodes(mainDB, orgDB, orgID); err != nil {
		return fmt.Errorf("oauth_authorization_codes: %w", err)
	}
	if err := copyRefreshTokens(mainDB, orgDB, orgID); err != nil {
		return fmt.Errorf("oauth_refresh_tokens: %w", err)
	}
	return nil
}

func copyAuthRequests(mainDB, orgDB *sql.DB, orgID string) error {
	return copyRows(mainDB, orgDB,
		`SELECT r.state, r.application_id, r.provider, r.code_verifier, r.return_url, r.created_at
		 FROM oauth_auth_requests r
		 JOIN applications a ON a.id = r.application_id
		 JOIN workspace w ON w.id = a.workspace_id
		 WHERE w.organization_id = ?`,
		`INSERT OR IGNORE INTO oauth_auth_requests
		 (state, application_id, provider, code_verifier, return_url, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		orgID,
	)
}

func copyAuthorizationCodes(mainDB, orgDB *sql.DB, orgID string) error {
	return copyRows(mainDB, orgDB,
		`SELECT c.code, c.client_row_id, c.application_id, c.user_id, c.redirect_uri,
		        c.scopes, c.code_challenge, c.code_challenge_method, c.nonce, c.created_at
		 FROM oauth_authorization_codes c
		 JOIN applications a ON a.id = c.application_id
		 JOIN workspace w ON w.id = a.workspace_id
		 WHERE w.organization_id = ?`,
		`INSERT OR IGNORE INTO oauth_authorization_codes
		 (code, client_row_id, application_id, user_id, redirect_uri,
		  scopes, code_challenge, code_challenge_method, nonce, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		orgID,
	)
}

func copyRefreshTokens(mainDB, orgDB *sql.DB, orgID string) error {
	return copyRows(mainDB, orgDB,
		`SELECT r.id, r.token_hash, r.client_row_id, r.application_id, r.user_id,
		        r.scopes, r.issued_at, r.expires_at, r.revoked_at, r.replaced_by_id
		 FROM oauth_refresh_tokens r
		 JOIN applications a ON a.id = r.application_id
		 JOIN workspace w ON w.id = a.workspace_id
		 WHERE w.organization_id = ?`,
		`INSERT OR IGNORE INTO oauth_refresh_tokens
		 (id, token_hash, client_row_id, application_id, user_id,
		  scopes, issued_at, expires_at, revoked_at, replaced_by_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		orgID,
	)
}

func dropGlobalOAuthTables(mainDB *sql.DB) error {
	stmts := []string{
		`DROP TABLE IF EXISTS oauth_refresh_tokens`,
		`DROP TABLE IF EXISTS oauth_authorization_codes`,
		`DROP TABLE IF EXISTS oauth_auth_requests`,
		`DROP TABLE IF EXISTS oauth_token_org_index`,
	}
	for _, stmt := range stmts {
		if _, err := mainDB.Exec(stmt); err != nil {
			return fmt.Errorf("%s: %w", stmt, err)
		}
	}
	return nil
}
