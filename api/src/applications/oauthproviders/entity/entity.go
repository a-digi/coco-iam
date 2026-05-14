// Package entity defines the domain types for the workspace-
// application OAuth provider feature (acting as OAuth CLIENT
// toward external IdPs like Google / GitHub / Microsoft).
package entity

import "errors"

// Provider is the enumeration of external IdP vendors we support.
// Callers should use the constants rather than raw strings so a
// typo at the boundary yields a compile error.
type Provider string

const (
	ProviderGoogle    Provider = "google"
	ProviderGitHub    Provider = "github"
	ProviderMicrosoft Provider = "microsoft"
)

// AllowedProviders is the whitelist used by admin handler
// validation on create / update. Keep in sync with the frontend
// dropdown in the Authentication tab.
var AllowedProviders = map[Provider]struct{}{
	ProviderGoogle:    {},
	ProviderGitHub:    {},
	ProviderMicrosoft: {},
}

// IsAllowedProvider reports whether p is one of the supported
// external IdPs. The frontend-side dropdown uses the same
// whitelist; this guard catches a drifted UI or a curl caller.
func IsAllowedProvider(p string) bool {
	_, ok := AllowedProviders[Provider(p)]
	return ok
}

// ProviderConfig is the decrypted row from
// application_oauth_providers. The repository layer decrypts the
// client secret on read so downstream callers never see the
// ciphertext directly.
type ProviderConfig struct {
	ID                string
	ApplicationID     string
	Provider          Provider
	ClientID          string
	ClientSecret      string
	DiscoveryURL      string
	AuthorizeURL      string
	TokenURL          string
	UserinfoURL       string
	Scopes            []string
	AllowLogin        bool
	AllowRegistration bool
	IsActive          bool
	CreatedAt         string
	UpdatedAt         string
}

// Identity is the canonical shape a provider-adapter returns
// after exchanging the authorization code + fetching the user's
// profile. Fields that are present for some providers and not
// others (EmailVerified is absent on some GitHub token types)
// are represented as zero values; the login decision layer
// handles the absence.
type Identity struct {
	Provider       Provider
	Sub            string
	Email          string
	EmailVerified  bool
	FirstName      string
	LastName       string
	PictureURL     string
}

// ErrProviderNotFound is returned by FindByID when the requested
// row does not exist for the caller's application.
var ErrProviderNotFound = errors.New("oauthproviders: provider not found")

// ErrDuplicateProvider is returned by Insert when the caller
// attempts to configure the same provider twice on the same
// application. Admins should PATCH the existing row instead.
var ErrDuplicateProvider = errors.New("oauthproviders: provider already configured for this application")
