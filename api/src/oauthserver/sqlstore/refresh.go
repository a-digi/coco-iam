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
	"github.com/google/uuid"
)

// RefreshRepo persists opaque refresh tokens. token_hash is the
// SHA-256 of the raw token; the raw value never leaves the
// process after Mint returns.
//
// Two modes:
//   - Single-DB (resolve == nil): all operations hit mainDB directly.
//     Used by tests and the legacy code path.
//   - Multi-DB (resolve != nil): each token is stored in the per-org
//     DB returned by resolve(orgID). knownOrgIDs enumerates all orgs
//     for scan-based routing. appResolve maps appID to orgID.
type RefreshRepo struct {
	mainDB      *sql.DB                             // sole DB in single-DB mode
	resolve     func(orgID string) (*sql.DB, error) // nil = single-DB mode
	knownOrgIDs func() []string                     // enumerates all org IDs for scan routing
	appResolve  func(appID string) (string, error)  // nil = use scan routing on Mint
	Now         func() time.Time
}

// NewRefreshRepo constructs a RefreshRepo over the given DB
// (single-DB mode). Kept for backward compatibility with tests.
func NewRefreshRepo(db *sql.DB) *RefreshRepo {
	return &RefreshRepo{mainDB: db, resolve: nil, Now: time.Now}
}

// NewRefreshRepoWithResolver returns a RefreshRepo that routes tokens
// to per-org DBs via the routing index in mainDB.
func NewRefreshRepoWithResolver(mainDB *sql.DB, resolve func(orgID string) (*sql.DB, error)) *RefreshRepo {
	return &RefreshRepo{mainDB: mainDB, resolve: resolve, Now: time.Now}
}

// NewRefreshRepoWithOrgResolver returns a RefreshRepo that routes tokens
// to per-org DBs via knownOrgIDs scan. appResolve maps appID to orgID on
// Mint. mainDB is not used in this mode.
func NewRefreshRepoWithOrgResolver(resolve func(orgID string) (*sql.DB, error), knownOrgIDs func() []string, appResolve func(appID string) (string, error)) *RefreshRepo {
	return &RefreshRepo{resolve: resolve, knownOrgIDs: knownOrgIDs, appResolve: appResolve, Now: time.Now}
}

// Mint generates a new refresh token, persists its hash, and
// returns the raw value the caller ships to the client.
func (r *RefreshRepo) Mint(ctx context.Context, clientRowID, applicationID, userID string, scopes []string, ttl time.Duration) (string, *entity.RefreshToken, error) {
	if ttl <= 0 {
		ttl = 14 * 24 * time.Hour
	}
	raw, err := tokenid.Generate(0)
	if err != nil {
		return "", nil, fmt.Errorf("sqlstore: refresh mint: %w", err)
	}
	hash := tokenid.Hash(raw)
	scopesJSON, _ := json.Marshal(scopes)
	id := uuid.New().String()
	now := r.Now().UTC()

	rt := &entity.RefreshToken{
		ID:            id,
		TokenHash:     hash,
		ClientRowID:   clientRowID,
		ApplicationID: applicationID,
		UserID:        userID,
		Scopes:        scopes,
		IssuedAt:      now.Format(time.RFC3339Nano),
		ExpiresAt:     now.Add(ttl).Format(time.RFC3339Nano),
	}

	if r.resolve != nil {
		// Multi-DB mode.
		var orgID string
		if r.appResolve != nil {
			var rerr error
			orgID, rerr = r.appResolve(applicationID)
			if rerr != nil {
				return "", nil, fmt.Errorf("sqlstore: lookup org for app %s: %w", applicationID, rerr)
			}
		} else if err := r.mainDB.QueryRowContext(ctx,
			`SELECT org_id FROM application_org_index WHERE application_id = ? LIMIT 1`,
			applicationID,
		).Scan(&orgID); err != nil {
			return "", nil, fmt.Errorf("sqlstore: lookup org for app %s: %w", applicationID, err)
		}
		orgDB, err := r.resolve(orgID)
		if err != nil {
			return "", nil, fmt.Errorf("sqlstore: open org db %s: %w", orgID, err)
		}
		_, err = orgDB.ExecContext(ctx,
			`INSERT INTO oauth_refresh_tokens
			 (id, token_hash, client_row_id, application_id, user_id, scopes, issued_at, expires_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			id, hash, clientRowID, applicationID, userID, string(scopesJSON),
			rt.IssuedAt, rt.ExpiresAt,
		)
		if err != nil {
			return "", nil, fmt.Errorf("sqlstore: insert refresh: %w", err)
		}
		return raw, rt, nil
	}

	// Single-DB mode.
	_, err = r.mainDB.ExecContext(ctx,
		`INSERT INTO oauth_refresh_tokens
		 (id, token_hash, client_row_id, application_id, user_id, scopes, issued_at, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, hash, clientRowID, applicationID, userID, string(scopesJSON),
		rt.IssuedAt, rt.ExpiresAt,
	)
	if err != nil {
		return "", nil, fmt.Errorf("sqlstore: insert refresh: %w", err)
	}
	return raw, rt, nil
}

// FindUnconsumed returns the row matching the raw token, but
// only if it's still active (not consumed, not expired).
// Detects replay: a row that's already revoked AND has a
// non-empty replaced_by_id signals "this raw value was used to
// rotate", and any further use returns ErrReplayDetected.
func (r *RefreshRepo) FindUnconsumed(ctx context.Context, raw string) (*entity.RefreshToken, error) {
	hash := tokenid.Hash(raw)

	tokenDB, err := r.tokenDBByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, entity.ErrRefreshNotFound) {
			return nil, entity.ErrRefreshNotFound
		}
		return nil, err
	}

	row := tokenDB.QueryRowContext(ctx,
		`SELECT id, token_hash, client_row_id, application_id, user_id, scopes,
		        issued_at, expires_at, revoked_at, replaced_by_id
		 FROM oauth_refresh_tokens WHERE token_hash = ? LIMIT 1`,
		hash,
	)
	rt, err := scanRefresh(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, entity.ErrRefreshNotFound
		}
		return nil, err
	}
	if rt.IsRevoked() && rt.ReplacedByID != "" {
		return nil, entity.ErrReplayDetected
	}
	if rt.IsRevoked() {
		return nil, entity.ErrRefreshNotFound
	}
	if expiry, err := time.Parse(time.RFC3339Nano, rt.ExpiresAt); err == nil {
		if r.Now().After(expiry) {
			return nil, entity.ErrRefreshNotFound
		}
	}
	return rt, nil
}

// Rotate marks the old token consumed and links the new
// token's id. Idempotency: a second call for the same oldID is
// a no-op (returns nil) so retries can't accidentally clear
// replaced_by_id.
//
// In multi-DB mode, the routing index maps token_hash → org_id,
// but Rotate receives token IDs (UUIDs). We fan out over all orgs
// that have entries in the routing index to find the owning org.
// This is O(N) where N = number of orgs, but Rotate is called at
// most once per token use.
func (r *RefreshRepo) Rotate(ctx context.Context, oldID, newID string) error {
	now := r.Now().UTC().Format(time.RFC3339Nano)

	tokenDB, err := r.tokenDBByID(ctx, oldID)
	if err != nil {
		// If we can't find it, treat as not-found and skip silently
		// (idempotent behaviour: already-rotated tokens are absent).
		if errors.Is(err, entity.ErrRefreshNotFound) {
			return nil
		}
		return fmt.Errorf("sqlstore: rotate find db: %w", err)
	}

	_, err = tokenDB.ExecContext(ctx,
		`UPDATE oauth_refresh_tokens
		 SET revoked_at = ?, replaced_by_id = ?
		 WHERE id = ? AND revoked_at IS NULL`,
		now, newID, oldID,
	)
	if err != nil {
		return fmt.Errorf("sqlstore: rotate refresh: %w", err)
	}
	return nil
}

// Revoke marks a refresh token consumed without issuing a
// replacement. Used by /oauth/revoke. Caller-supplied raw value
// → hash lookup; missing rows are tolerated (idempotent).
func (r *RefreshRepo) Revoke(ctx context.Context, raw string) error {
	hash := tokenid.Hash(raw)
	now := r.Now().UTC().Format(time.RFC3339Nano)

	tokenDB, err := r.tokenDBByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, entity.ErrRefreshNotFound) {
			return nil // idempotent
		}
		return fmt.Errorf("sqlstore: revoke find db: %w", err)
	}

	_, err = tokenDB.ExecContext(ctx,
		`UPDATE oauth_refresh_tokens SET revoked_at = ?
		 WHERE token_hash = ? AND revoked_at IS NULL`,
		now, hash,
	)
	if err != nil {
		return fmt.Errorf("sqlstore: revoke refresh: %w", err)
	}
	return nil
}

// RevokeFamily walks the rotation chain rooted at any member
// id and revokes every entry. Used when Rotate detects a
// replay — the whole lineage is compromised so we burn it all.
//
// In multi-DB mode, we fan out over all orgs in the routing
// index to find the owning org (same approach as Rotate).
func (r *RefreshRepo) RevokeFamily(ctx context.Context, anyMemberID string) error {
	tokenDB, err := r.tokenDBByID(ctx, anyMemberID)
	if err != nil {
		if errors.Is(err, entity.ErrRefreshNotFound) {
			return nil // idempotent
		}
		return fmt.Errorf("sqlstore: revoke family find db: %w", err)
	}

	tx, err := tokenDB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("sqlstore: family begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	// Climb up.
	root := anyMemberID
	for i := 0; i < 256; i++ {
		var parent string
		err := tx.QueryRowContext(ctx,
			`SELECT id FROM oauth_refresh_tokens WHERE replaced_by_id = ? LIMIT 1`,
			root,
		).Scan(&parent)
		if errors.Is(err, sql.ErrNoRows) {
			break
		}
		if err != nil {
			return fmt.Errorf("sqlstore: walk up: %w", err)
		}
		if parent == "" || parent == root {
			break
		}
		root = parent
	}

	// Walk down marking everything revoked.
	now := r.Now().UTC().Format(time.RFC3339Nano)
	current := root
	for i := 0; i < 256; i++ {
		if _, err := tx.ExecContext(ctx,
			`UPDATE oauth_refresh_tokens SET revoked_at = ?
			 WHERE id = ? AND revoked_at IS NULL`,
			now, current,
		); err != nil {
			return fmt.Errorf("sqlstore: revoke node: %w", err)
		}
		var next sql.NullString
		err := tx.QueryRowContext(ctx,
			`SELECT replaced_by_id FROM oauth_refresh_tokens WHERE id = ? LIMIT 1`,
			current,
		).Scan(&next)
		if errors.Is(err, sql.ErrNoRows) || !next.Valid || next.String == "" {
			break
		}
		if err != nil {
			return fmt.Errorf("sqlstore: walk down: %w", err)
		}
		current = next.String
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("sqlstore: family commit: %w", err)
	}
	return nil
}

// -- internal routing helpers -----------------------------------------

// tokenDBByHash returns the DB that owns the token with the given hash.
// In single-DB mode it returns mainDB. In multi-DB mode it scans all
// known per-org oauth DBs.
func (r *RefreshRepo) tokenDBByHash(ctx context.Context, hash string) (*sql.DB, error) {
	if r.resolve == nil {
		return r.mainDB, nil
	}
	if r.knownOrgIDs != nil {
		for _, orgID := range r.knownOrgIDs() {
			db, err := r.resolve(orgID)
			if err != nil || db == nil {
				continue
			}
			var found string
			if db.QueryRowContext(ctx,
				`SELECT token_hash FROM oauth_refresh_tokens WHERE token_hash = ? LIMIT 1`, hash,
			).Scan(&found) == nil {
				return db, nil
			}
		}
	}
	return nil, entity.ErrRefreshNotFound
}

// tokenDBByID finds the org DB that holds a refresh token with the
// given ID by scanning all known per-org oauth DBs.
//
// In single-DB mode returns mainDB immediately.
func (r *RefreshRepo) tokenDBByID(ctx context.Context, id string) (*sql.DB, error) {
	if r.resolve == nil {
		return r.mainDB, nil
	}

	var orgIDs []string
	if r.knownOrgIDs != nil {
		orgIDs = r.knownOrgIDs()
	}

	for _, orgID := range orgIDs {
		db, err := r.resolve(orgID)
		if err != nil {
			continue
		}
		var found int
		if err := db.QueryRowContext(ctx,
			`SELECT 1 FROM oauth_refresh_tokens WHERE id = ? LIMIT 1`, id,
		).Scan(&found); err == nil && found == 1 {
			return db, nil
		}
	}
	return nil, entity.ErrRefreshNotFound
}

func scanRefresh(row rowScanner) (*entity.RefreshToken, error) {
	var (
		rt          entity.RefreshToken
		scopesRaw   string
		revokedRaw  sql.NullString
		replacedRaw sql.NullString
	)
	err := row.Scan(
		&rt.ID, &rt.TokenHash, &rt.ClientRowID, &rt.ApplicationID, &rt.UserID,
		&scopesRaw, &rt.IssuedAt, &rt.ExpiresAt, &revokedRaw, &replacedRaw,
	)
	if err != nil {
		return nil, err
	}
	if scopesRaw != "" {
		_ = json.Unmarshal([]byte(scopesRaw), &rt.Scopes)
	}
	if revokedRaw.Valid {
		rt.RevokedAt = revokedRaw.String
	}
	if replacedRaw.Valid {
		rt.ReplacedByID = replacedRaw.String
	}
	return &rt, nil
}

var _ oauthserver.RefreshStore = (*RefreshRepo)(nil)
