// Package entity carries the oauthserver's plain-data domain
// types. No I/O, no framework deps, no coco-iam deps — everything
// here is safe to expose as the public API of a future extracted
// library.
package entity

import (
	"errors"
	"strings"
)

// ClientType discriminates OAuth client registrations. Public
// clients can't protect a secret (typically SPAs / native apps)
// and MUST use PKCE. Confidential clients authenticate with a
// client secret and MAY use PKCE in addition.
type ClientType string

const (
	ClientTypePublic       ClientType = "public"
	ClientTypeConfidential ClientType = "confidential"
)

// Client is the persisted row for one registered OAuth client.
// Per-application: a given client_id is unique inside its
// application_id scope but may recur across applications.
type Client struct {
	// ID is the opaque primary key we mint on insert. Callers
	// never see this outside the server internals — OAuth
	// messages reference ClientID instead.
	ID              string
	ApplicationID   string
	ClientID        string
	// SecretHash is bcrypt-hashed; empty for public clients.
	SecretHash      string
	Type            ClientType
	DisplayName     string
	RedirectURIs    []string
	AllowedScopes   []string
	RequireConsent  bool
	AccessTokenTTL  int // seconds
	RefreshTokenTTL int // seconds
	IsActive        bool
	CreatedAt       string
	UpdatedAt       string
}

// PermitsRedirect reports whether the given redirect_uri exactly
// matches one of the registered ones. The comparison is
// case-sensitive on scheme/host per RFC 3986 and full-string on
// the path — no wildcards, no suffix matching.
func (c *Client) PermitsRedirect(uri string) bool {
	for _, reg := range c.RedirectURIs {
		if reg == uri {
			return true
		}
	}
	return false
}

// IsPublic is the common guard for PKCE-required branches.
func (c *Client) IsPublic() bool {
	return c.Type == ClientTypePublic
}

// AuthorizationCode is the short-lived row minted when the user
// consents. On /token exchange it's verified + consumed.
type AuthorizationCode struct {
	Code                string
	ClientRowID         string
	ApplicationID       string
	UserID              string
	RedirectURI         string
	Scopes              []string
	CodeChallenge       string
	CodeChallengeMethod string
	Nonce               string
	CreatedAt           string
}

// RefreshToken is the persisted record (server side) for an
// opaque refresh token. The token value the client holds is
// never stored — we keep its SHA-256 so a DB snapshot can't
// leak usable credentials.
type RefreshToken struct {
	ID            string
	TokenHash     string
	ClientRowID   string
	ApplicationID string
	UserID        string
	Scopes        []string
	IssuedAt      string
	ExpiresAt     string
	RevokedAt     string
	ReplacedByID  string
}

// IsRevoked reports whether the row has been marked revoked or
// replaced. Handlers treat both as "unusable".
func (r *RefreshToken) IsRevoked() bool {
	return strings.TrimSpace(r.RevokedAt) != ""
}

// Consent is the per-(user, client) record of the last consent
// decision. The handler loads one per authorize call to decide
// whether the consent screen can be skipped.
type Consent struct {
	ID            string
	UserID        string
	ClientRowID   string
	GrantedScopes []string
	GrantedAt     string
	RevokedAt     string
}

// IsRevoked reports whether the consent record has been
// explicitly rescinded.
func (c *Consent) IsRevoked() bool {
	return strings.TrimSpace(c.RevokedAt) != ""
}

// Sentinel errors callers (handlers + adapters) can branch on.
// The wire-level OAuth errors (RFC 6749 §5.2) are modelled as
// OAuthError below so handlers can translate Go errors → spec
// errors without losing information.
var (
	ErrClientNotFound   = errors.New("oauthserver: client not found")
	ErrCodeNotFound     = errors.New("oauthserver: authorization code not found or already used")
	ErrRefreshNotFound  = errors.New("oauthserver: refresh token not found")
	ErrConsentNotFound  = errors.New("oauthserver: consent record not found")
	ErrUserNotFound     = errors.New("oauthserver: user not found")
	ErrDuplicateClient  = errors.New("oauthserver: client with that client_id already registered for this application")
	ErrReplayDetected   = errors.New("oauthserver: refresh token replay detected")
)

// OAuthErrorCode matches the values in RFC 6749 §5.2 + RFC 6750.
type OAuthErrorCode string

const (
	ErrCodeInvalidRequest          OAuthErrorCode = "invalid_request"
	ErrCodeInvalidClient           OAuthErrorCode = "invalid_client"
	ErrCodeInvalidGrant            OAuthErrorCode = "invalid_grant"
	ErrCodeUnauthorizedClient      OAuthErrorCode = "unauthorized_client"
	ErrCodeUnsupportedGrantType    OAuthErrorCode = "unsupported_grant_type"
	ErrCodeInvalidScope            OAuthErrorCode = "invalid_scope"
	ErrCodeAccessDenied            OAuthErrorCode = "access_denied"
	ErrCodeServerError             OAuthErrorCode = "server_error"
	ErrCodeTemporarilyUnavailable  OAuthErrorCode = "temporarily_unavailable"
	ErrCodeUnsupportedResponseType OAuthErrorCode = "unsupported_response_type"
)

// OAuthError is what /authorize and /token handlers surface when
// the request violates the spec. The outer error.Error() is the
// Description so tests can assert on string contents.
type OAuthError struct {
	Code        OAuthErrorCode
	Description string
	Status      int // HTTP status to use for /token; /authorize uses redirect instead
}

// Error implements the error interface.
func (e *OAuthError) Error() string {
	if e.Description == "" {
		return string(e.Code)
	}
	return string(e.Code) + ": " + e.Description
}

// NewOAuthError is the constructor that handlers + adapters use.
// Kept small because the error shape is fixed by the spec.
func NewOAuthError(code OAuthErrorCode, description string, status int) *OAuthError {
	return &OAuthError{Code: code, Description: description, Status: status}
}

// OAuthErrorResponse is the RFC 6749 §5.2 error body written to the wire.
// Used exclusively in swag @Failure annotations.
type OAuthErrorResponse struct {
	Error            string `json:"error"             example:"invalid_request"`
	ErrorDescription string `json:"error_description" example:"grant_type required"`
}
