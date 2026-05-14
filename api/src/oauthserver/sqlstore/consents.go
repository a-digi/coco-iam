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
	"github.com/google/uuid"
)

// ConsentRepo persists per-(user, client) consent decisions.
// The schema lives in the per-org users DB so consent records
// move with the user when the org is migrated.
//
// ConsentRepo takes a DBResolver instead of a single *sql.DB
// because the per-org DB is selected per request (the
// organization id flows in from the authenticated user). The
// resolver is the same shape oauthproviders/login uses for the
// SQLLinker — keeps the wiring consistent.
type ConsentRepo struct {
	Resolve DBResolver
	Now     func() time.Time
}

// DBResolver returns the per-org users DB for the given
// organization id. Wiring constructs one per session by closing
// over the OrgUserDBRegistry; tests pass a stub that returns a
// shared in-memory DB.
type DBResolver func(orgID string) (*sql.DB, error)

// NewConsentRepo wraps the resolver as a ConsentStore.
func NewConsentRepo(resolve DBResolver) *ConsentRepo {
	return &ConsentRepo{Resolve: resolve, Now: time.Now}
}

func (r *ConsentRepo) dbFor(orgID string) (*sql.DB, error) {
	if r.Resolve == nil {
		return nil, errors.New("sqlstore: nil consent resolver")
	}
	db, err := r.Resolve(orgID)
	if err != nil {
		return nil, fmt.Errorf("sqlstore: resolve org %s: %w", orgID, err)
	}
	if db == nil {
		return nil, fmt.Errorf("sqlstore: org %s has no DB", orgID)
	}
	return db, nil
}

// Load returns the most-recent non-revoked consent for the
// (user, client) pair. Missing → ErrConsentNotFound.
func (r *ConsentRepo) Load(ctx context.Context, organizationID, userID, clientRowID string) (*entity.Consent, error) {
	db, err := r.dbFor(organizationID)
	if err != nil {
		return nil, err
	}
	row := db.QueryRowContext(ctx,
		`SELECT id, user_id, client_row_id, granted_scopes, granted_at, revoked_at
		 FROM oauth_user_consents
		 WHERE user_id = ? AND client_row_id = ?
		 LIMIT 1`,
		userID, clientRowID,
	)
	var (
		c          entity.Consent
		scopesRaw  string
		revokedRaw sql.NullString
	)
	if err := row.Scan(&c.ID, &c.UserID, &c.ClientRowID, &scopesRaw, &c.GrantedAt, &revokedRaw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, entity.ErrConsentNotFound
		}
		return nil, fmt.Errorf("sqlstore: scan consent: %w", err)
	}
	if scopesRaw != "" {
		_ = json.Unmarshal([]byte(scopesRaw), &c.GrantedScopes)
	}
	if revokedRaw.Valid {
		c.RevokedAt = revokedRaw.String
	}
	if c.IsRevoked() {
		return nil, entity.ErrConsentNotFound
	}
	return &c, nil
}

// Record persists or replaces the consent for (user, client).
// Replaces because a fresh authorize call with a wider scope
// set should overwrite the previous record — admin UI exposes
// the latest set, not a history.
func (r *ConsentRepo) Record(ctx context.Context, organizationID, userID, clientRowID string, scopes []string) error {
	db, err := r.dbFor(organizationID)
	if err != nil {
		return err
	}
	now := r.Now().UTC().Format(time.RFC3339)
	scopesJSON, _ := json.Marshal(scopes)
	// Upsert via INSERT OR REPLACE on the unique (user, client)
	// index. On replace we mint a fresh id so revocation chains
	// don't accidentally re-bind to a stale record.
	_, err = db.ExecContext(ctx,
		`INSERT INTO oauth_user_consents
		 (id, user_id, client_row_id, granted_scopes, granted_at, revoked_at)
		 VALUES (?, ?, ?, ?, ?, NULL)
		 ON CONFLICT(user_id, client_row_id) DO UPDATE SET
		   id = excluded.id,
		   granted_scopes = excluded.granted_scopes,
		   granted_at = excluded.granted_at,
		   revoked_at = NULL`,
		uuid.New().String(), userID, clientRowID,
		string(scopesJSON), now,
	)
	if err != nil {
		return fmt.Errorf("sqlstore: upsert consent: %w", err)
	}
	return nil
}

// Revoke marks the consent rescinded. The next authorize call
// for (user, client) will force the consent screen.
func (r *ConsentRepo) Revoke(ctx context.Context, organizationID, userID, clientRowID string) error {
	db, err := r.dbFor(organizationID)
	if err != nil {
		return err
	}
	now := r.Now().UTC().Format(time.RFC3339)
	_, err = db.ExecContext(ctx,
		`UPDATE oauth_user_consents SET revoked_at = ?
		 WHERE user_id = ? AND client_row_id = ? AND revoked_at IS NULL`,
		now, userID, clientRowID,
	)
	if err != nil {
		return fmt.Errorf("sqlstore: revoke consent: %w", err)
	}
	return nil
}

// compile-time check
var _ oauthserver.ConsentStore = (*ConsentRepo)(nil)
