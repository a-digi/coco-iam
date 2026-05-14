// Package oauthserverwiring is the coco-iam glue between the
// pure oauthserver library and the concrete services this
// repo owns (bcrypt, the main DB, the per-org users DB, the
// per-app RSA key service, the existing login page). Code
// here stays behind when the oauthserver library is extracted;
// a downstream consumer writes their own equivalents.
package oauthserverwiring

import (
	"errors"

	"github.com/a-digi/coco-iam/src/oauthserver/sqlstore"
	"golang.org/x/crypto/bcrypt"
)

// BcryptHasher adapts the stdlib-adjacent bcrypt package to the
// oauthserver sqlstore SecretHasher interface.
type BcryptHasher struct {
	// Cost is the bcrypt cost factor. 10 matches the
	// default bcrypt package constant and the value used
	// elsewhere in coco-iam (admin password hashing).
	Cost int
}

// NewBcryptHasher returns a ready-to-use hasher with the
// project-standard cost factor. Pass cost <= 0 to get bcrypt's
// default.
func NewBcryptHasher(cost int) *BcryptHasher {
	if cost <= 0 {
		cost = bcrypt.DefaultCost
	}
	return &BcryptHasher{Cost: cost}
}

// Hash implements sqlstore.SecretHasher.
func (h *BcryptHasher) Hash(plain string) (string, error) {
	out, err := bcrypt.GenerateFromPassword([]byte(plain), h.Cost)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// Verify implements sqlstore.SecretHasher. Returns a generic
// error on mismatch; callers collapse every failure to
// invalid_client on the wire.
func (h *BcryptHasher) Verify(hashed, plain string) error {
	if err := bcrypt.CompareHashAndPassword([]byte(hashed), []byte(plain)); err != nil {
		return errors.New("oauthserverwiring: bcrypt mismatch")
	}
	return nil
}

// Compile-time check: *BcryptHasher satisfies the
// sqlstore.SecretHasher interface.
var _ sqlstore.SecretHasher = (*BcryptHasher)(nil)
