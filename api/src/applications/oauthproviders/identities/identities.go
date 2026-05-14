// Package identities wraps the per-org user_oauth_identities
// table. A single row binds one of our workspace-app users.id
// values to an external IdP identity pair (provider, sub).
//
// The repo is read + write in one type because the surface is
// small; the authorize/callback handshake uses it for the full
// "find / create / link" decision path.
package identities

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Repository wraps the per-org profile/users database's
// user_oauth_identities table. Construct one per-request via
// New(perOrgDB) — the same *sql.DB the org's users table lives
// in.
type Repository struct {
	db *sql.DB
}

// New constructs a Repository over the given per-org *sql.DB.
func New(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Identity is the row shape. CreatedAt is kept as string so the
// wire / DB representation stays consistent with the rest of the
// codebase.
type Identity struct {
	ID                   string
	UserID               string
	Provider             string
	ProviderSub          string
	EmailAtLink          string
	EmailVerifiedAtLink  bool
	CreatedAt            string
}

// ErrIdentityNotFound is returned when FindByProviderSub / the
// user-id lookups find no row.
var ErrIdentityNotFound = errors.New("identities: not found")

// FindByProviderSub resolves a `(provider, sub)` pair to its
// identity row. The primary lookup on the OAuth callback path.
func (r *Repository) FindByProviderSub(provider, sub string) (*Identity, error) {
	row := r.db.QueryRow(
		`SELECT id, user_id, provider, provider_sub,
		        email_at_link, email_verified_at_link, created_at
		 FROM user_oauth_identities
		 WHERE provider = ? AND provider_sub = ?
		 LIMIT 1`,
		provider, sub,
	)
	return scanOne(row)
}

// ListForUser returns every identity linked to the given user
// id. Empty slice when the user has none. Used by the admin UI
// "linked accounts" panel (future work).
func (r *Repository) ListForUser(userID string) ([]Identity, error) {
	rows, err := r.db.Query(
		`SELECT id, user_id, provider, provider_sub,
		        email_at_link, email_verified_at_link, created_at
		 FROM user_oauth_identities
		 WHERE user_id = ?
		 ORDER BY created_at ASC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("identities: list: %w", err)
	}
	defer rows.Close()

	out := []Identity{}
	for rows.Next() {
		var id Identity
		var verified int
		if err := rows.Scan(
			&id.ID, &id.UserID, &id.Provider, &id.ProviderSub,
			&id.EmailAtLink, &verified, &id.CreatedAt,
		); err != nil {
			return nil, err
		}
		id.EmailVerifiedAtLink = verified != 0
		out = append(out, id)
	}
	return out, rows.Err()
}

// LinkInput carries a create request — one identity added to
// an existing user. The id is minted here.
type LinkInput struct {
	UserID              string
	Provider            string
	ProviderSub         string
	EmailAtLink         string
	EmailVerifiedAtLink bool
}

// Link persists a new identity row. Returns the stored Identity.
// The unique index on (provider, provider_sub) keeps each
// external identity owned by exactly one of our users — a
// duplicate Link returns the underlying SQL error; callers
// should pre-check with FindByProviderSub.
func (r *Repository) Link(in LinkInput) (Identity, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	id := uuid.New().String()
	verified := 0
	if in.EmailVerifiedAtLink {
		verified = 1
	}
	_, err := r.db.Exec(
		`INSERT INTO user_oauth_identities
		 (id, user_id, provider, provider_sub, email_at_link, email_verified_at_link, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, in.UserID, in.Provider, in.ProviderSub,
		in.EmailAtLink, verified, now,
	)
	if err != nil {
		return Identity{}, fmt.Errorf("identities: insert: %w", err)
	}
	return Identity{
		ID:                  id,
		UserID:              in.UserID,
		Provider:            in.Provider,
		ProviderSub:         in.ProviderSub,
		EmailAtLink:         in.EmailAtLink,
		EmailVerifiedAtLink: in.EmailVerifiedAtLink,
		CreatedAt:           now,
	}, nil
}

// Unlink removes a (user, provider) binding. Used by a future
// "disconnect this IdP" flow; included now so the schema's
// lifecycle is covered end-to-end.
func (r *Repository) Unlink(userID, provider string) error {
	_, err := r.db.Exec(
		`DELETE FROM user_oauth_identities WHERE user_id = ? AND provider = ?`,
		userID, provider,
	)
	if err != nil {
		return fmt.Errorf("identities: delete: %w", err)
	}
	return nil
}

// scanOne wraps the single-row Scan boilerplate so the two
// single-row lookups don't repeat the error plumbing.
func scanOne(row *sql.Row) (*Identity, error) {
	var id Identity
	var verified int
	if err := row.Scan(
		&id.ID, &id.UserID, &id.Provider, &id.ProviderSub,
		&id.EmailAtLink, &verified, &id.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrIdentityNotFound
		}
		return nil, err
	}
	id.EmailVerifiedAtLink = verified != 0
	return &id, nil
}
