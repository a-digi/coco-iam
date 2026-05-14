// Package entity defines the on-disk row shape for application API
// credentials. These rows live in the per-organization
// `api_credentials.db` file; see ../dbregistry for the storage
// boundary.
package entity

import "time"

// Credential is one machine-auth credential issued to an application.
// `APIID` is the opaque public identifier sent on the wire (safe to
// log); `SecretHash` is the bcrypt hash of the plaintext secret and
// is never returned to any caller outside the single create response
// that issued it.
type Credential struct {
	_             struct{} `table:"application_api_credentials"`
	ID            string   `db:"id" dbtype:"TEXT" nullable:"false" json:"id"`
	ApplicationID string   `db:"application_id" dbtype:"TEXT" nullable:"false" json:"application_id"`
	APIID         string   `db:"api_id" dbtype:"TEXT" nullable:"false" json:"api_id"`
	SecretHash    string   `db:"secret_hash" dbtype:"TEXT" nullable:"false" json:"-"`
	Label         string   `db:"label" dbtype:"TEXT" nullable:"false" default:"" json:"label"`
	// Purposes is the JSON-encoded `[]string` of purposes this
	// credential may be used for. Handlers check that their required
	// purpose is present before acting on the request.
	Purposes   string     `db:"purposes" dbtype:"TEXT" nullable:"false" default:"[]" json:"-"`
	ExpiresAt  time.Time  `db:"expires_at" dbtype:"DATETIME" nullable:"false" json:"expires_at"`
	IsActive   bool       `db:"is_active" dbtype:"BOOLEAN" nullable:"false" default:"true" json:"is_active"`
	LastUsedAt *time.Time `db:"last_used_at" dbtype:"DATETIME" nullable:"true" json:"last_used_at"`
	CreatedAt  time.Time  `db:"created_at" dbtype:"DATETIME" nullable:"true" json:"created_at"`
	RevokedAt  *time.Time `db:"revoked_at" dbtype:"DATETIME" nullable:"true" json:"revoked_at"`
}
