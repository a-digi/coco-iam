// Package authn holds the decision logic that turns an HTTP Basic
// Authorization header into an authorised Credential for a given
// application + required purpose. The handler glue in the sibling
// `public` package is a thin wrapper around AuthenticateBasicAuth —
// all the business rules live here so they're trivially unit-testable.
package authn

import (
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/a-digi/coco-iam/src/applications/apicredentials/entity"
	"github.com/a-digi/coco-iam/src/applications/apicredentials/purpose"
	"github.com/a-digi/coco-iam/src/auth/crypto/bcrypt"
)

// ErrUnauthorized is the single error returned for any authentication
// or authorization failure. Handlers translate this to a generic 401
// so the endpoint can't be used as an oracle ("is this a real
// api_id?" / "is this credential revoked?" / "which purpose is
// missing?" are all indistinguishable from the caller's side).
var ErrUnauthorized = errors.New("api-credentials: unauthorized")

// CredentialLookup is the narrow interface authn needs from storage.
// In production this is backed by the repository; in tests it's a
// struct literal with a canned response.
type CredentialLookup interface {
	FindByAPIID(apiID string) (*entity.Credential, []string, error)
}

// AuthenticateBasicAuth parses an `Authorization: Basic ...` header,
// looks up the credential, verifies the secret with bcrypt, and
// applies the full stack of authorization checks:
//
//   - credential belongs to the expected applicationID
//   - credential is_active
//   - credential not yet past expires_at
//   - required purpose is in the credential's purpose list
//
// Any failure — missing header, bad scheme, malformed base64, unknown
// api_id, wrong secret, wrong application, revoked, expired, missing
// purpose — returns (nil, ErrUnauthorized). On success returns the
// matched credential so the caller can stamp last_used_at (fire-and-
// forget); callers MUST NOT pass the SecretHash back in any response.
//
// `now` is injected so tests can assert expiry handling without
// sleeping.
func AuthenticateBasicAuth(
	header string,
	expectedApplicationID string,
	required purpose.Purpose,
	now time.Time,
	lookup CredentialLookup,
) (*entity.Credential, error) {
	apiID, secret, ok := parseBasicAuth(header)
	if !ok {
		return nil, ErrUnauthorized
	}

	cred, purposes, err := lookup.FindByAPIID(apiID)
	if err != nil || cred == nil {
		return nil, ErrUnauthorized
	}

	if cred.ApplicationID != expectedApplicationID {
		// Cross-tenant: credential exists but was issued to a
		// different application. Reject generically.
		return nil, ErrUnauthorized
	}
	if !cred.IsActive {
		return nil, ErrUnauthorized
	}
	if !cred.ExpiresAt.IsZero() && !now.Before(cred.ExpiresAt) {
		return nil, ErrUnauthorized
	}
	if !hasPurpose(purposes, required) {
		return nil, ErrUnauthorized
	}

	if err := bcrypt.VerifyPassword(cred.SecretHash, secret); err != nil {
		return nil, ErrUnauthorized
	}

	return cred, nil
}

// parseBasicAuth returns the decoded (username, password, true) for a
// well-formed `Authorization: Basic …` header, and (_, _, false)
// otherwise. Strict: unknown schemes, malformed base64, or a missing
// colon all fail closed.
func parseBasicAuth(header string) (apiID, secret string, ok bool) {
	if header == "" {
		return "", "", false
	}
	const prefix = "Basic "
	if !strings.HasPrefix(header, prefix) {
		return "", "", false
	}
	payload := strings.TrimSpace(header[len(prefix):])
	if payload == "" {
		return "", "", false
	}
	decoded, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return "", "", false
	}
	idx := strings.IndexByte(string(decoded), ':')
	if idx < 0 {
		return "", "", false
	}
	return string(decoded[:idx]), string(decoded[idx+1:]), true
}

// hasPurpose is a linear search — purposes lists are tiny (typically
// one element today, a handful in the future) so a set would be more
// machinery than signal.
func hasPurpose(purposes []string, required purpose.Purpose) bool {
	want := required.String()
	for _, p := range purposes {
		if p == want {
			return true
		}
	}
	return false
}

// HeaderFromRequest extracts the Authorization header from an
// *http.Request. Kept as a helper so callers don't depend on the net/
// http package directly in their business logic.
func HeaderFromRequest(r *http.Request) string {
	if r == nil {
		return ""
	}
	return r.Header.Get("Authorization")
}
