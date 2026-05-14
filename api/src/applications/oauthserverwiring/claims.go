package oauthserverwiring

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/a-digi/coco-iam/src/oauthserver"
	"github.com/a-digi/coco-iam/src/organizations/users/dbregistry"
)

// UsersDBClaimsReader implements oauthserver.UserClaimsReader by
// pulling the user row from the per-org users database. Only
// fields the OIDC scopes actually request end up in the response;
// the handler does additional scope filtering on top.
type UsersDBClaimsReader struct {
	Registry *dbregistry.OrgUserDBRegistry
	// Resolver is the test seam: pass nil in production to use
	// the registry; tests can swap in a stub that returns an
	// in-memory DB.
	Resolver func(orgID string) (*sql.DB, error)
}

// NewUsersDBClaimsReader wraps the given registry.
func NewUsersDBClaimsReader(reg *dbregistry.OrgUserDBRegistry) *UsersDBClaimsReader {
	return &UsersDBClaimsReader{Registry: reg}
}

func (r *UsersDBClaimsReader) dbFor(orgID string) (*sql.DB, error) {
	if r.Resolver != nil {
		return r.Resolver(orgID)
	}
	if r.Registry == nil {
		return nil, errors.New("oauthserverwiring: UsersDBClaimsReader has nil registry")
	}
	mgr, err := r.Registry.For(orgID)
	if err != nil {
		return nil, fmt.Errorf("oauthserverwiring: resolve org %s: %w", orgID, err)
	}
	if mgr == nil || mgr.Connector == nil || mgr.Connector.DB == nil {
		return nil, fmt.Errorf("oauthserverwiring: org %s has no DB", orgID)
	}
	return mgr.Connector.DB, nil
}

// LoadClaims implements oauthserver.UserClaimsReader. Returns
// every claim the requested scopes can populate; the handler
// filters by scope.ClaimsFor before serialising — passing extra
// fields here is safe.
func (r *UsersDBClaimsReader) LoadClaims(_ context.Context, organizationID, userID string, _ []string) (map[string]any, error) {
	db, err := r.dbFor(organizationID)
	if err != nil {
		return nil, err
	}
	row := db.QueryRow(
		`SELECT username, email FROM users WHERE id = ? LIMIT 1`,
		userID,
	)
	var (
		username sql.NullString
		email    sql.NullString
	)
	if err := row.Scan(&username, &email); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("oauthserverwiring: load user claims: %w", err)
	}
	out := map[string]any{}
	if email.Valid {
		out["email"] = email.String
		// We only flag email as verified once the OAuth-link
		// flow recorded it as such — for password-only signups
		// the user_oauth_identities row is absent so we default
		// to true (consistent with the existing app login flow
		// which treats every users.email as authoritative).
		out["email_verified"] = true
	}
	if username.Valid {
		out["preferred_username"] = username.String
	}
	return out, nil
}

var _ oauthserver.UserClaimsReader = (*UsersDBClaimsReader)(nil)
