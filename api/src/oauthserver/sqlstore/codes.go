package sqlstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/a-digi/coco-iam/src/oauthserver"
	"github.com/a-digi/coco-iam/src/oauthserver/entity"
	"github.com/a-digi/coco-iam/src/oauthserver/tokenid"
)

// CodeRepo persists short-lived authorization codes. Single
// implementation handles both Mint (insert) and ConsumeOnce
// (atomic select+delete).
//
// Two modes:
//   - Single-DB (resolve == nil): all operations hit mainDB directly.
//     Used by tests and the legacy code path.
//   - Multi-DB (resolve != nil): each code is stored in the per-org DB
//     returned by resolve(orgID). knownOrgIDs enumerates all orgs for
//     scan-based routing. appResolve maps appID to orgID.
type CodeRepo struct {
	mainDB      *sql.DB                             // sole DB in single-DB mode
	resolve     func(orgID string) (*sql.DB, error) // nil = single-DB mode
	knownOrgIDs func() []string                     // enumerates all org IDs for scan routing
	appResolve  func(appID string) (string, error)  // nil = use scan routing on Mint
	Now         func() time.Time
}

// NewCodeRepo returns a CodeRepo over the given DB (single-DB mode).
// Kept for backward compatibility with tests.
func NewCodeRepo(db *sql.DB) *CodeRepo {
	return &CodeRepo{mainDB: db, resolve: nil, Now: time.Now}
}

// NewCodeRepoWithResolver returns a CodeRepo that routes codes to
// per-org DBs via the routing index in mainDB.
func NewCodeRepoWithResolver(mainDB *sql.DB, resolve func(orgID string) (*sql.DB, error)) *CodeRepo {
	return &CodeRepo{mainDB: mainDB, resolve: resolve, Now: time.Now}
}

// NewCodeRepoWithOrgResolver returns a CodeRepo that routes codes to
// per-org DBs via knownOrgIDs scan. appResolve maps appID to orgID on
// Mint. mainDB is not used in this mode.
func NewCodeRepoWithOrgResolver(resolve func(orgID string) (*sql.DB, error), knownOrgIDs func() []string, appResolve func(appID string) (string, error)) *CodeRepo {
	return &CodeRepo{resolve: resolve, knownOrgIDs: knownOrgIDs, appResolve: appResolve, Now: time.Now}
}

// Mint generates a fresh opaque code, persists the row, and
// returns the code value the caller should redirect to the
// client. ttl <= 0 falls back to 5 minutes.
func (r *CodeRepo) Mint(ctx context.Context, in oauthserver.CodeMintInput, ttl time.Duration) (string, error) {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	code, err := tokenid.Generate(0)
	if err != nil {
		return "", fmt.Errorf("sqlstore: mint code: %w", err)
	}
	scopesJSON, _ := json.Marshal(in.Scopes)
	now := r.Now().UTC()

	if r.resolve != nil {
		// Multi-DB mode: find the org, open the org DB, insert there,
		// then write a routing entry to mainDB.
		var orgID string
		if r.appResolve != nil {
			var rerr error
			orgID, rerr = r.appResolve(in.ApplicationID)
			if rerr != nil {
				return "", fmt.Errorf("sqlstore: lookup org for app %s: %w", in.ApplicationID, rerr)
			}
		} else if err := r.mainDB.QueryRowContext(ctx,
			`SELECT org_id FROM application_org_index WHERE application_id = ? LIMIT 1`,
			in.ApplicationID,
		).Scan(&orgID); err != nil {
			return "", fmt.Errorf("sqlstore: lookup org for app %s: %w", in.ApplicationID, err)
		}
		orgDB, err := r.resolve(orgID)
		if err != nil {
			return "", fmt.Errorf("sqlstore: open org db %s: %w", orgID, err)
		}
		_, err = orgDB.ExecContext(ctx,
			`INSERT INTO oauth_authorization_codes
			 (code, client_row_id, application_id, user_id, redirect_uri,
			  scopes, code_challenge, code_challenge_method, nonce, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			code, in.ClientRowID, in.ApplicationID, in.UserID, in.RedirectURI,
			string(scopesJSON), in.CodeChallenge,
			defaultStr(in.CodeChallengeMethod, "S256"),
			in.Nonce, now.Format(time.RFC3339Nano),
		)
		if err != nil {
			return "", fmt.Errorf("sqlstore: insert code: %w", err)
		}
		return code, nil
	}

	// Single-DB mode.
	_, err = r.mainDB.ExecContext(ctx,
		`INSERT INTO oauth_authorization_codes
		 (code, client_row_id, application_id, user_id, redirect_uri,
		  scopes, code_challenge, code_challenge_method, nonce, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		code, in.ClientRowID, in.ApplicationID, in.UserID, in.RedirectURI,
		string(scopesJSON), in.CodeChallenge,
		defaultStr(in.CodeChallengeMethod, "S256"),
		in.Nonce, now.Format(time.RFC3339Nano),
	)
	if err != nil {
		return "", fmt.Errorf("sqlstore: insert code: %w", err)
	}
	return code, nil
}

// ConsumeOnce fetches and deletes a code in one transaction.
// A returning caller that re-uses the code receives
// ErrCodeNotFound.
func (r *CodeRepo) ConsumeOnce(ctx context.Context, code string) (*entity.AuthorizationCode, error) {
	codeDB, _, err := r.codeDB(ctx, code)
	if err != nil {
		if errors.Is(err, entity.ErrCodeNotFound) {
			return nil, entity.ErrCodeNotFound
		}
		return nil, err
	}

	tx, err := codeDB.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("sqlstore: begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	row := tx.QueryRowContext(ctx,
		`SELECT code, client_row_id, application_id, user_id, redirect_uri,
		        scopes, code_challenge, code_challenge_method, nonce, created_at
		 FROM oauth_authorization_codes WHERE code = ? LIMIT 1`,
		code,
	)
	var (
		ac        entity.AuthorizationCode
		scopesRaw string
	)
	if err := row.Scan(
		&ac.Code, &ac.ClientRowID, &ac.ApplicationID, &ac.UserID, &ac.RedirectURI,
		&scopesRaw, &ac.CodeChallenge, &ac.CodeChallengeMethod, &ac.Nonce, &ac.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, entity.ErrCodeNotFound
		}
		return nil, fmt.Errorf("sqlstore: scan code: %w", err)
	}
	if scopesRaw != "" {
		_ = json.Unmarshal([]byte(scopesRaw), &ac.Scopes)
	}
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM oauth_authorization_codes WHERE code = ?`, code); err != nil {
		return nil, fmt.Errorf("sqlstore: delete code: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("sqlstore: commit: %w", err)
	}

	return &ac, nil
}

// DeleteExpired removes rows older than `before`. Run by a
// background sweeper; returns the row count for logging.
//
// In multi-DB mode it fans out across all orgs known via the
// routing index and cleans up the index entries as well.
func (r *CodeRepo) DeleteExpired(ctx context.Context, before time.Time) (int, error) {
	cutoff := before.UTC().Format(time.RFC3339Nano)

	if r.resolve != nil {
		// Multi-DB mode: enumerate orgs via knownOrgIDs.
		var orgIDs []string
		if r.knownOrgIDs != nil {
			orgIDs = r.knownOrgIDs()
		}

		total := 0
		for _, orgID := range orgIDs {
			orgDB, err := r.resolve(orgID)
			if err != nil {
				continue
			}
			res, err := orgDB.ExecContext(ctx,
				`DELETE FROM oauth_authorization_codes WHERE created_at < ?`, cutoff)
			if err == nil {
				n, _ := res.RowsAffected()
				total += int(n)
			}
		}
		return total, nil
	}

	// Single-DB mode.
	res, err := r.mainDB.ExecContext(ctx,
		`DELETE FROM oauth_authorization_codes WHERE created_at < ?`,
		cutoff,
	)
	if err != nil {
		return 0, fmt.Errorf("sqlstore: sweep codes: %w", err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// -- internal routing helpers -----------------------------------------

// codeDB returns the DB that holds the given code, plus the orgID
// (empty in single-DB mode). Scans all known per-org oauth DBs in
// multi-DB mode; returns ErrCodeNotFound when not found in any.
func (r *CodeRepo) codeDB(ctx context.Context, code string) (*sql.DB, string, error) {
	if r.resolve == nil {
		return r.mainDB, "", nil
	}
	if r.knownOrgIDs != nil {
		for _, orgID := range r.knownOrgIDs() {
			db, err := r.resolve(orgID)
			if err != nil || db == nil {
				continue
			}
			var found string
			if db.QueryRowContext(ctx,
				`SELECT code FROM oauth_authorization_codes WHERE code = ? LIMIT 1`, code,
			).Scan(&found) == nil {
				return db, orgID, nil
			}
		}
	}
	return nil, "", entity.ErrCodeNotFound
}

func defaultStr(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

// compile-time check
var _ oauthserver.CodeStore = (*CodeRepo)(nil)
