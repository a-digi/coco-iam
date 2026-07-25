// Package totp is a domain-agnostic RFC 6238 TOTP helper. It knows
// nothing about admin_users, organizations, or applications — secret
// generation, provisioning-URI construction, and code validation are
// pure functions over a secret string. Callers own persistence
// (encrypt-at-rest, e.g. via secretbox) and own which population the
// secret belongs to.
//
// The actual time-step/HMAC comparison in Validate is delegated to
// github.com/pquerna/otp — reinventing that math by hand is not worth
// the risk versus a well-audited library. Secret and provisioning-URI
// generation are simple enough to own directly.
package totp

import (
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"net/url"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// secretBytes is 160 bits — RFC 4226 §4 / RFC 6238's recommended
// minimum key length for HMAC-SHA1-based OTP.
const secretBytes = 20

// Period is the RFC 6238 time-step, in seconds. 30s is the de facto
// standard every authenticator app (Google Authenticator, Authy, 1Password)
// assumes.
const Period = 30

// Digits is the code length shown to the user and expected back.
const Digits = 6

// GenerateSecret returns a new random base32-encoded (no padding,
// uppercase) TOTP secret, suitable for both QR-code provisioning and
// manual entry into an authenticator app.
func GenerateSecret() (string, error) {
	raw := make([]byte, secretBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("totp: generate secret: %w", err)
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw), nil
}

// ProvisioningURI builds the otpauth:// URI an authenticator app scans
// (as a QR code) or accepts via manual entry. issuer and accountLabel
// are both shown to the user inside their app — issuer should be a
// fixed, recognizable string (e.g. "coco-iam"); accountLabel should
// identify which account this is (e.g. the admin's email).
func ProvisioningURI(secret, issuer, accountLabel string) string {
	u := url.URL{
		Scheme: "otpauth",
		Host:   "totp",
		Path:   "/" + issuer + ":" + accountLabel,
	}
	q := url.Values{}
	q.Set("secret", secret)
	q.Set("issuer", issuer)
	q.Set("algorithm", "SHA1")
	q.Set("digits", fmt.Sprintf("%d", Digits))
	q.Set("period", fmt.Sprintf("%d", Period))
	u.RawQuery = q.Encode()
	return u.String()
}

// Validate reports whether code is a valid TOTP code for secret at
// the current time, tolerating up to skew time-steps of clock drift
// in either direction (skew=1 means the previous, current, and next
// 30s window all count — do not widen this casually, it directly
// trades security for convenience).
func Validate(secret, code string, skew uint) bool {
	ok, err := totp.ValidateCustom(code, secret, time.Now().UTC(), totp.ValidateOpts{
		Period:    Period,
		Skew:      skew,
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})
	if err != nil {
		return false
	}
	return ok
}
