package oauthserver

import (
	"context"
	"time"

	"github.com/a-digi/coco-iam/src/oauthserver/entity"
)

// The interfaces the handler layer depends on. A library
// consumer implements each one against their own stack; the
// handlers stay framework- and database-agnostic.
//
// Error contract: every method that can fail to find a row
// returns a sentinel from the entity package (ErrClientNotFound
// / ErrCodeNotFound / ErrRefreshNotFound / ErrConsentNotFound /
// ErrUserNotFound / ErrReplayDetected). Other errors indicate
// transient / infrastructure failure and surface as server_error
// on the wire.

// ClientRegistry resolves registered OAuth clients. FindByClientID
// takes the (application_id, client_id) pair because client_id is
// scoped per-application in our schema — two apps may register the
// same public client_id independently.
type ClientRegistry interface {
	FindByClientID(ctx context.Context, applicationID, clientID string) (*entity.Client, error)
	// VerifySecret returns nil on a correct match. Implementations
	// MUST compare hashes in constant time. Public clients (no
	// secret) should reject any non-empty submission.
	VerifySecret(ctx context.Context, client *entity.Client, submittedSecret string) error
}

// CodeStore persists short-lived authorization codes.
// MintInput wraps the fields the authorize handler has validated;
// the adapter is responsible for generating the opaque code value
// and returning it.
type CodeMintInput struct {
	ClientRowID         string
	ApplicationID       string
	UserID              string
	RedirectURI         string
	Scopes              []string
	CodeChallenge       string
	CodeChallengeMethod string
	Nonce               string
}

type CodeStore interface {
	Mint(ctx context.Context, in CodeMintInput, ttl time.Duration) (code string, err error)
	// ConsumeOnce atomically fetches and deletes a code row. A
	// returning-caller that re-uses a code gets ErrCodeNotFound.
	ConsumeOnce(ctx context.Context, code string) (*entity.AuthorizationCode, error)
	DeleteExpired(ctx context.Context, before time.Time) (removed int, err error)
}

// RefreshStore persists opaque refresh tokens.
// Mint generates the raw token, returns it to the caller so it
// can ship it down the wire, and persists the hash. Consume
// looks the row up by its hash, validates expiry, and marks it
// consumed for future replay detection.
type RefreshStore interface {
	Mint(ctx context.Context, clientRowID, applicationID, userID string, scopes []string, ttl time.Duration) (raw string, record *entity.RefreshToken, err error)
	// FindUnconsumed returns the record iff it exists, is not
	// yet consumed, and hasn't expired. Any fault state returns
	// ErrRefreshNotFound.
	FindUnconsumed(ctx context.Context, raw string) (*entity.RefreshToken, error)
	// Rotate marks the old record consumed and links it to the
	// new record's id. A later second-use of the old token is
	// caught via the replaced_by_id chain and reported as
	// ErrReplayDetected.
	Rotate(ctx context.Context, oldID, newID string) error
	// Revoke marks a specific refresh token consumed WITHOUT
	// issuing a replacement. Called by the revocation handler.
	Revoke(ctx context.Context, raw string) error
	// RevokeFamily marks every refresh row derived from the
	// given refresh-id's lineage as revoked. Called when a
	// replay is detected (the whole chain is compromised).
	RevokeFamily(ctx context.Context, anyMemberID string) error
}

// ConsentStore persists the user's consent decisions. Lookup is
// by (user, client); absence means "never consented" and the
// authorize handler shows the consent screen.
type ConsentStore interface {
	Load(ctx context.Context, organizationID, userID, clientRowID string) (*entity.Consent, error)
	Record(ctx context.Context, organizationID, userID, clientRowID string, scopes []string) error
	Revoke(ctx context.Context, organizationID, userID, clientRowID string) error
}

// UserClaimsReader supplies OIDC claim values for a user. The
// handler passes the scope list the client was granted; the
// reader decides which claims to include (honouring the
// standard OIDC scope → claim mapping plus any application-
// specific extensions).
type UserClaimsReader interface {
	LoadClaims(ctx context.Context, organizationID, userID string, grantedScopes []string) (map[string]any, error)
}

// TokenSigner wraps the per-application RS256 signing
// primitive. The library is format-agnostic so the adapter
// picks the claim-shaping + `kid` header handling it needs.
type TokenSigner interface {
	SignAccessToken(ctx context.Context, applicationID string, claims map[string]any) (string, error)
	SignIDToken(ctx context.Context, applicationID string, claims map[string]any) (string, error)
}

// UserAuthenticator wraps the "is the caller logged in?" check
// the authorize handler runs before deciding whether to redirect
// to the login page. Implementations look at a session cookie /
// header, verify it, and return the user's id.
//
// CurrentUser returns ("", nil) when no session is present — a
// distinct signal from an error, because "not logged in" is a
// normal state the handler redirects to the login flow.
type UserAuthenticator interface {
	CurrentUser(ctx context.Context, request RequestInfo) (userID, organizationID string, err error)
}

// RequestInfo is the tiny view of an HTTP request the
// authenticator needs. Keeping it narrow means the library
// compiles without a full http.Request dep in the ports file,
// which helps future-extraction.
type RequestInfo struct {
	CookieValue string // value of the session cookie, if any
	Header      string // bearer header, if the authenticator uses one
}
