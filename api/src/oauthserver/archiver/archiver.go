// Package archiver sweeps per-org oauth.db files every 10 minutes,
// moving expired and consumed token rows into dated SQLite archive
// files under data/db/organization/<orgID>/oauth/archives/.
//
// Archive DB layout:
//
//	data/db/organization/<orgID>/oauth/archives/YYYY-MM-DD_archive.db
//
// Each archive DB has the same three table schemas as oauth.db.
// Rows are copied with INSERT OR IGNORE then deleted from the source,
// so a crash between the two leaves the row in the source and the
// next sweep retries cleanly.
package archiver

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/a-digi/coco-iam/src/oauthserver/dbregistry"
	"github.com/a-digi/coco-logger/logger"
	_ "github.com/mattn/go-sqlite3"
)

const (
	sweepInterval        = 10 * time.Minute
	authRequestTTL       = 10 * time.Minute
	authCodeTTL          = 5 * time.Minute
	archiveSubDir        = "oauth/archives"
)

// Archiver periodically moves expired/consumed token rows from each
// org's oauth.db into a dated archive file.
type Archiver struct {
	reg     *dbregistry.OrgOAuthDBRegistry
	baseDir string
	log     logger.Logger
}

// New constructs an Archiver.
func New(reg *dbregistry.OrgOAuthDBRegistry, baseDir string, log logger.Logger) *Archiver {
	return &Archiver{reg: reg, baseDir: baseDir, log: log}
}

// Run blocks, ticking every sweepInterval, until ctx is cancelled.
// Intended to be launched as a goroutine.
func (a *Archiver) Run(ctx context.Context) {
	ticker := time.NewTicker(sweepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := a.Sweep(); err != nil {
				a.log.Warning("oauth archiver: sweep error: %v", err)
			}
		}
	}
}

// Sweep iterates over all known org IDs and archives stale rows from
// each org's oauth.db. Errors for individual orgs are logged but do
// not abort the remaining orgs.
func (a *Archiver) Sweep() error {
	orgIDs := a.reg.KnownOrgIDs()
	now := time.Now().UTC()
	for _, orgID := range orgIDs {
		mgr, err := a.reg.For(orgID)
		if err != nil || mgr == nil || mgr.Connector == nil {
			a.log.Warning("oauth archiver: open org db %s: %v", orgID, err)
			continue
		}
		if err := a.sweepOrg(mgr.Connector.DB, orgID, now); err != nil {
			a.log.Warning("oauth archiver: org %s: %v", orgID, err)
		}
	}
	return nil
}

func (a *Archiver) sweepOrg(orgDB *sql.DB, orgID string, now time.Time) error {
	archiveDB, err := a.openArchiveDB(orgID, now)
	if err != nil {
		return fmt.Errorf("open archive db: %w", err)
	}
	defer archiveDB.Close()

	if err := ensureArchiveSchema(archiveDB); err != nil {
		return fmt.Errorf("ensure schema: %w", err)
	}

	authReqCutoff := now.Add(-authRequestTTL).UTC().Format(time.RFC3339Nano)
	codeCutoff := now.Add(-authCodeTTL).UTC().Format(time.RFC3339Nano)
	expiredCutoff := now.UTC().Format(time.RFC3339Nano)

	if err := archiveRows(orgDB, archiveDB,
		`SELECT state, application_id, provider, code_verifier, return_url, created_at
		 FROM oauth_auth_requests WHERE created_at < ?`,
		`INSERT OR IGNORE INTO oauth_auth_requests
		 (state, application_id, provider, code_verifier, return_url, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		`DELETE FROM oauth_auth_requests WHERE created_at < ?`,
		authReqCutoff,
	); err != nil {
		return fmt.Errorf("archive auth_requests: %w", err)
	}

	if err := archiveRows(orgDB, archiveDB,
		`SELECT code, client_row_id, application_id, user_id, redirect_uri,
		        scopes, code_challenge, code_challenge_method, nonce, created_at
		 FROM oauth_authorization_codes WHERE created_at < ?`,
		`INSERT OR IGNORE INTO oauth_authorization_codes
		 (code, client_row_id, application_id, user_id, redirect_uri,
		  scopes, code_challenge, code_challenge_method, nonce, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		`DELETE FROM oauth_authorization_codes WHERE created_at < ?`,
		codeCutoff,
	); err != nil {
		return fmt.Errorf("archive authorization_codes: %w", err)
	}

	// Archive refresh tokens that are revoked OR expired.
	if err := archiveRows(orgDB, archiveDB,
		`SELECT id, token_hash, client_row_id, application_id, user_id,
		        scopes, issued_at, expires_at, revoked_at, replaced_by_id
		 FROM oauth_refresh_tokens WHERE revoked_at IS NOT NULL OR expires_at < ?`,
		`INSERT OR IGNORE INTO oauth_refresh_tokens
		 (id, token_hash, client_row_id, application_id, user_id,
		  scopes, issued_at, expires_at, revoked_at, replaced_by_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		`DELETE FROM oauth_refresh_tokens WHERE revoked_at IS NOT NULL OR expires_at < ?`,
		expiredCutoff,
	); err != nil {
		return fmt.Errorf("archive refresh_tokens: %w", err)
	}

	return nil
}

// archiveRows copies rows matching selectSQL (with one arg) into the
// archive DB, then deletes them from the source using deleteSQL (same
// arg). Runs inside a transaction on the archive DB.
func archiveRows(src, dst *sql.DB, selectSQL, insertSQL, deleteSQL, arg string) error {
	rows, err := src.Query(selectSQL, arg)
	if err != nil {
		return fmt.Errorf("select: %w", err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("columns: %w", err)
	}

	tx, err := dst.Begin()
	if err != nil {
		return fmt.Errorf("begin archive tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	for rows.Next() {
		vals := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return fmt.Errorf("scan: %w", err)
		}
		if _, err := tx.Exec(insertSQL, vals...); err != nil {
			return fmt.Errorf("insert archive: %w", err)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("rows: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit archive: %w", err)
	}

	// Delete from source only after archive commit succeeds.
	if _, err := src.Exec(deleteSQL, arg); err != nil {
		return fmt.Errorf("delete source: %w", err)
	}
	return nil
}

func (a *Archiver) openArchiveDB(orgID string, t time.Time) (*sql.DB, error) {
	dir := filepath.Join(a.baseDir, "organization", orgID, archiveSubDir)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, fmt.Errorf("mkdir archive dir: %w", err)
	}
	name := t.Format("2006-01-02") + "_archive.db"
	path := filepath.Join(dir, name)
	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open archive db: %w", err)
	}
	return db, nil
}

func ensureArchiveSchema(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS oauth_auth_requests (
		    state          TEXT NOT NULL PRIMARY KEY,
		    application_id TEXT NOT NULL,
		    provider       TEXT NOT NULL,
		    code_verifier  TEXT NOT NULL,
		    return_url     TEXT NOT NULL DEFAULT '',
		    created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS oauth_authorization_codes (
		    code                  TEXT NOT NULL PRIMARY KEY,
		    client_row_id         TEXT NOT NULL,
		    application_id        TEXT NOT NULL,
		    user_id               TEXT NOT NULL,
		    redirect_uri          TEXT NOT NULL,
		    scopes                TEXT NOT NULL DEFAULT '[]',
		    code_challenge        TEXT NOT NULL DEFAULT '',
		    code_challenge_method TEXT NOT NULL DEFAULT 'S256',
		    nonce                 TEXT NOT NULL DEFAULT '',
		    created_at            DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS oauth_refresh_tokens (
		    id             TEXT NOT NULL PRIMARY KEY,
		    token_hash     TEXT NOT NULL UNIQUE,
		    client_row_id  TEXT NOT NULL,
		    application_id TEXT NOT NULL,
		    user_id        TEXT NOT NULL,
		    scopes         TEXT NOT NULL DEFAULT '[]',
		    issued_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		    expires_at     DATETIME NOT NULL,
		    revoked_at     DATETIME,
		    replaced_by_id TEXT
		)`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("create table: %w", err)
		}
	}
	return nil
}
