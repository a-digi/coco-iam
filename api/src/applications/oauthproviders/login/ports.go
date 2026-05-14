// Package login hosts the workspace-application OAuth login
// handshake — the authorize + callback handlers + the pure
// decision layer that resolves an IdP identity to one of our
// users (login, link, register).
//
// The package declares its collaborators as narrow interfaces
// (UserLinker, adapters.IdentityResolver, TokenIssuer) so
// handler tests can substitute fakes without touching the DB
// or the real IdP servers.
package login

import (
	"context"
	"errors"

	"github.com/a-digi/coco-iam/src/applications/oauthproviders/adapters"
	"github.com/a-digi/coco-iam/src/applications/oauthproviders/entity"
)

// Errors surfaced by ResolveLogin so handler tests can branch
// precisely. The handler maps them to HTTP codes:
//   ErrRegistrationClosed   → 403 "sign-up is closed"
//   ErrUntrustedEmail       → 403 "could not verify IdP email"
//   ErrLoginDisabled        → 403 "login via this provider is off"
//   ErrProviderInactive     → 404-ish; provider exists but disabled
var (
	ErrRegistrationClosed = errors.New("login: registration not allowed")
	ErrUntrustedEmail     = errors.New("login: IdP did not return a verified email")
	ErrLoginDisabled      = errors.New("login: provider is not enabled for login")
	ErrProviderInactive   = errors.New("login: provider row is inactive")
)

// ResolveMode tells the caller which path ResolveLogin took so
// handlers can log / surface a first-login welcome.
type ResolveMode string

const (
	ModeLogin      ResolveMode = "login"      // existing identity row matched
	ModeLinked     ResolveMode = "linked"     // existing user, new identity attached
	ModeRegistered ResolveMode = "registered" // new user created from IdP claims
)

// AppSettings is the slice of application configuration the
// decision layer needs. Passed in as a value so the pure
// function can be tested without the DB.
type AppSettings struct {
	ApplicationID     string
	OrganizationID    string
	AllowRegistration bool
}

// UserLinker is the single seam the decision layer uses to
// touch per-org storage. Implemented in production by the
// sqlLinker struct; tests stub it with a map-backed fake.
type UserLinker interface {
	// FindByIdentity returns our user_id for a given
	// (provider, sub) pair. Returns ok=false when no link
	// exists for that identity.
	FindByIdentity(orgID string, provider entity.Provider, sub string) (userID string, ok bool, err error)
	// FindByEmail returns our user_id whose primary email
	// matches. Used by the auto-link branch; ok=false when
	// no match.
	FindByEmail(orgID, email string) (userID string, ok bool, err error)
	// CreateUserFromIdentity persists a new users row + a
	// matching identity row in one atomic write. Returns the
	// new user id.
	CreateUserFromIdentity(orgID string, id entity.Identity) (userID string, err error)
	// LinkIdentity attaches a new identity to an existing
	// user. Idempotent: calling twice with the same input is
	// a no-op.
	LinkIdentity(orgID, userID string, id entity.Identity) error
}

// ResolveLogin is the register-or-link-or-login decision. Pure:
// the only side-effects are the UserLinker writes, and those
// happen only on the branches that explicitly call for them.
//
// Flow:
//  1. FindByIdentity(provider, sub) → hit → ModeLogin.
//  2. Otherwise, if the identity carries a verified email:
//     a. FindByEmail → hit → LinkIdentity → ModeLinked.
//     b. FindByEmail → miss:
//        - registration allowed: CreateUserFromIdentity → ModeRegistered.
//        - registration not allowed: ErrRegistrationClosed.
//  3. Email missing or unverified:
//     - registration allowed: CreateUserFromIdentity (no email
//       match possible, stored with email_verified=false).
//     - otherwise: ErrUntrustedEmail.
func ResolveLogin(id entity.Identity, cfg entity.ProviderConfig, app AppSettings, linker UserLinker) (string, ResolveMode, error) {
	if !cfg.IsActive {
		return "", "", ErrProviderInactive
	}
	if !cfg.AllowLogin {
		return "", "", ErrLoginDisabled
	}
	userID, ok, err := linker.FindByIdentity(app.OrganizationID, id.Provider, id.Sub)
	if err != nil {
		return "", "", err
	}
	if ok {
		return userID, ModeLogin, nil
	}
	canRegister := cfg.AllowRegistration && app.AllowRegistration
	hasVerifiedEmail := id.Email != "" && id.EmailVerified

	if hasVerifiedEmail {
		userID, ok, err := linker.FindByEmail(app.OrganizationID, id.Email)
		if err != nil {
			return "", "", err
		}
		if ok {
			if err := linker.LinkIdentity(app.OrganizationID, userID, id); err != nil {
				return "", "", err
			}
			return userID, ModeLinked, nil
		}
		if !canRegister {
			return "", "", ErrRegistrationClosed
		}
		newID, err := linker.CreateUserFromIdentity(app.OrganizationID, id)
		if err != nil {
			return "", "", err
		}
		return newID, ModeRegistered, nil
	}

	// No verified email from the IdP.
	if !canRegister {
		return "", "", ErrUntrustedEmail
	}
	newID, err := linker.CreateUserFromIdentity(app.OrganizationID, id)
	if err != nil {
		return "", "", err
	}
	return newID, ModeRegistered, nil
}

// TokenIssuer is the seam over the existing oauth.IssueAppLoginTokens
// helper. The handler layer depends on this interface so tests can
// substitute a fake that returns canned tokens without spinning up
// a key service.
type TokenIssuer interface {
	IssueLoginTokens(ctx context.Context, appID, userID string, scopes []string, resourceIDs map[string][]string) (accessToken, refreshToken string, err error)
}

// Resolver is the adapter the handlers talk to. Alias kept here
// so handler files don't also import the adapters package
// directly — the login package is the one that composes them.
type Resolver = adapters.IdentityResolver
