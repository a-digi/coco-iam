package login

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/a-digi/coco-iam/src/applications/oauthproviders/entity"
	"github.com/a-digi/coco-iam/src/applications/oauthproviders/identities"
	"github.com/a-digi/coco-iam/src/organizations/users/dbregistry"
	"github.com/google/uuid"
)

// DBResolver returns the per-org users DB for a given
// organisation id. Production wiring supplies a resolver that
// closes over the OrgUserDBRegistry; tests swap in a stub that
// returns a pre-prepared in-memory DB so they don't need the
// migration folder machinery.
type DBResolver func(orgID string) (*sql.DB, error)

// SQLLinker is the production UserLinker. It runs small targeted
// queries against the `users` + `user_oauth_identities` tables
// (both live in the per-org users DB). All DB access goes
// through Resolve so tests can inject a fake DB.
type SQLLinker struct {
	Resolve DBResolver
}

// NewSQLLinker wraps the given registry as a UserLinker.
func NewSQLLinker(reg *dbregistry.OrgUserDBRegistry) *SQLLinker {
	return &SQLLinker{Resolve: resolverFromRegistry(reg)}
}

func resolverFromRegistry(reg *dbregistry.OrgUserDBRegistry) DBResolver {
	return func(orgID string) (*sql.DB, error) {
		if reg == nil {
			return nil, errors.New("sqllinker: nil registry")
		}
		mgr, err := reg.For(orgID)
		if err != nil {
			return nil, fmt.Errorf("sqllinker: resolve org %s: %w", orgID, err)
		}
		if mgr == nil || mgr.Connector == nil || mgr.Connector.DB == nil {
			return nil, fmt.Errorf("sqllinker: org %s has no DB", orgID)
		}
		return mgr.Connector.DB, nil
	}
}

func (l *SQLLinker) dbFor(orgID string) (*sql.DB, error) {
	if l.Resolve == nil {
		return nil, errors.New("sqllinker: nil resolver")
	}
	return l.Resolve(orgID)
}

// FindByIdentity resolves (provider, sub) → user_id via the
// identities table.
func (l *SQLLinker) FindByIdentity(orgID string, provider entity.Provider, sub string) (string, bool, error) {
	db, err := l.dbFor(orgID)
	if err != nil {
		return "", false, err
	}
	repo := identities.New(db)
	id, err := repo.FindByProviderSub(string(provider), sub)
	if err != nil {
		if errors.Is(err, identities.ErrIdentityNotFound) {
			return "", false, nil
		}
		return "", false, err
	}
	return id.UserID, true, nil
}

// FindByEmail resolves email → user_id via the users table.
// Email comparison is case-insensitive to match the signup
// convention elsewhere in the codebase.
func (l *SQLLinker) FindByEmail(orgID, email string) (string, bool, error) {
	db, err := l.dbFor(orgID)
	if err != nil {
		return "", false, err
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return "", false, nil
	}
	var userID string
	err = db.QueryRow(
		`SELECT id FROM users WHERE LOWER(email) = ? LIMIT 1`,
		email,
	).Scan(&userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("sqllinker: find by email: %w", err)
	}
	return userID, true, nil
}

// CreateUserFromIdentity persists a new users row + its
// identity row atomically. The user gets is_active=1 and
// must_change_password=0 — OAuth users never need a password.
// Email/username are taken from the identity; when the IdP
// didn't supply an email, the username falls back to
// "<provider>:<sub>" so we always have something unique.
func (l *SQLLinker) CreateUserFromIdentity(orgID string, id entity.Identity) (string, error) {
	db, err := l.dbFor(orgID)
	if err != nil {
		return "", err
	}
	userID := uuid.New().String()
	now := time.Now().UTC().Format(time.RFC3339)

	username := pickUsername(id)
	email := strings.ToLower(strings.TrimSpace(id.Email))

	tx, err := db.Begin()
	if err != nil {
		return "", fmt.Errorf("sqllinker: begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.Exec(
		`INSERT INTO users (id, username, email, is_active, must_change_password, created_at)
		 VALUES (?, ?, ?, 1, 0, ?)`,
		userID, username, email, now,
	); err != nil {
		return "", fmt.Errorf("sqllinker: insert user: %w", err)
	}
	verified := 0
	if id.EmailVerified {
		verified = 1
	}
	if _, err := tx.Exec(
		`INSERT INTO user_oauth_identities
		 (id, user_id, provider, provider_sub, email_at_link, email_verified_at_link, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		uuid.New().String(), userID, string(id.Provider), id.Sub, email, verified, now,
	); err != nil {
		return "", fmt.Errorf("sqllinker: insert identity: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("sqllinker: commit: %w", err)
	}
	return userID, nil
}

// LinkIdentity attaches an identity to an existing user.
// Idempotent: a pre-existing (user, provider) binding is left
// alone and we return success.
func (l *SQLLinker) LinkIdentity(orgID, userID string, id entity.Identity) error {
	db, err := l.dbFor(orgID)
	if err != nil {
		return err
	}
	repo := identities.New(db)

	// Idempotency: if the same user already has this
	// (provider, sub) linked, skip.
	existing, err := repo.FindByProviderSub(string(id.Provider), id.Sub)
	if err == nil && existing != nil && existing.UserID == userID {
		return nil
	}
	_, err = repo.Link(identities.LinkInput{
		UserID:              userID,
		Provider:            string(id.Provider),
		ProviderSub:         id.Sub,
		EmailAtLink:         id.Email,
		EmailVerifiedAtLink: id.EmailVerified,
	})
	return err
}

// pickUsername picks a unique, human-readable username from the
// identity claims. Preference order: first.last → email local
// part → provider:sub fallback.
func pickUsername(id entity.Identity) string {
	if f, l := strings.TrimSpace(id.FirstName), strings.TrimSpace(id.LastName); f != "" || l != "" {
		if f != "" && l != "" {
			return strings.ToLower(f) + "." + strings.ToLower(l)
		}
		return strings.ToLower(f + l)
	}
	if id.Email != "" {
		if i := strings.Index(id.Email, "@"); i > 0 {
			return strings.ToLower(id.Email[:i])
		}
	}
	return string(id.Provider) + ":" + id.Sub
}
