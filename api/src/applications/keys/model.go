// Package keys owns the per-application RSA signing keys. Multiple
// keys can coexist for one application; the lifecycle is:
//
//   pending → active → deactivated → expired
//
// Only the `active` key signs newly-issued tokens. Deactivated keys
// keep verifying (via JWKS) until their `expires_at` passes — by
// default 24 hours after deactivation. PEM files are never deleted
// from disk; expiry is a query-time filter.
//
// The admin login surface is a separate feature and continues to sign
// with the shared HS256 secret from `config.json`; none of this
// package is touched from that path.
package keys

import (
	"errors"
	"time"
)

// ContextBagKeyService is the DI key under which the Service is
// registered at server startup.
const ContextBagKeyService = "applications.keys.Service"

// StorageBaseDir is the path (relative to the process wd) under which
// per-organization folders live. Key files land at
// `<StorageBaseDir>/organization/<org_id>/auth/<application_id>/<kid>/{private,public}.pem`.
// Rooting the keys inside the per-org folder means the deletion-
// consumer that archives `<StorageBaseDir>/organization/<org_id>/`
// picks up the key material automatically.
const StorageBaseDir = "data/db"

// LegacyStorageSubdir is the pre-migration location: `data/appkeys/`
// held a flat `<application_id>/<kid>/` tree with no org scoping.
// FileStore.MigrateFromLegacy walks this on boot and moves surviving
// folders into the new layout; this constant is retained so the
// migration keeps working on a cold checkout.
const LegacyStorageSubdir = "data/appkeys"

// KeySize is the modulus bit-length for generated RSA keys. RS256 is
// the JWT industry default; 2048 is the widely-supported floor.
const KeySize = 2048

// DeactivatedGrace is how long a deactivated key remains verifiable
// after it stops signing. 24 hours balances "clients have time to
// refresh their JWKS cache" with "stolen keys stop working fast".
const DeactivatedGrace = 24 * time.Hour

// KeyStatus is the lifecycle marker stored in the DB. One active + at
// most one pending + any number of deactivated keys may coexist for
// a single application.
type KeyStatus string

const (
	StatusPending     KeyStatus = "pending"
	StatusActive      KeyStatus = "active"
	StatusDeactivated KeyStatus = "deactivated"
)

// Canonical errors the service returns to handlers.
var (
	ErrNotFound         = errors.New("keys: keypair not found")
	ErrPendingExists    = errors.New("keys: a pending key already exists for this application — discard or accept it first")
	ErrNoPending        = errors.New("keys: no pending key for this application")
	ErrNotDeactivated   = errors.New("keys: only deactivated keys can be force-expired")
	ErrKeyNotVerifiable = errors.New("keys: key is expired or unknown")
)

// KeyRow is the DB-backed metadata for one key. PEM bytes live on
// disk, not in the DB.
type KeyRow struct {
	ID            string     `json:"id"`
	ApplicationID string     `json:"application_id"`
	Status        KeyStatus  `json:"status"`
	CreatedAt     time.Time  `json:"created_at"`
	ActivatedAt   *time.Time `json:"activated_at,omitempty"`
	DeactivatedAt *time.Time `json:"deactivated_at,omitempty"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
}

// IsVerifiable returns true when the key can still be used to verify
// existing tokens — i.e., it's active, or it's deactivated but
// `expires_at` is still in the future.
func (r KeyRow) IsVerifiable(now time.Time) bool {
	switch r.Status {
	case StatusActive:
		return true
	case StatusDeactivated:
		return r.ExpiresAt != nil && r.ExpiresAt.After(now)
	default:
		return false
	}
}

// Keypair is the shape the admin UI receives. `PrivatePEM` is only
// populated for callers holding `applications:keys:read_private`.
type Keypair struct {
	ID            string     `json:"id"`           // the JWT `kid`
	ApplicationID string     `json:"application_id"`
	Status        KeyStatus  `json:"status"`
	PublicPEM     string     `json:"public_pem"`
	PrivatePEM    string     `json:"private_pem,omitempty"`
	HasPrivate    bool       `json:"has_private"`
	CreatedAt     time.Time  `json:"created_at"`
	ActivatedAt   *time.Time `json:"activated_at,omitempty"`
	DeactivatedAt *time.Time `json:"deactivated_at,omitempty"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
}
