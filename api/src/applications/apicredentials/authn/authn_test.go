package authn

import (
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/a-digi/coco-iam/src/applications/apicredentials/entity"
	"github.com/a-digi/coco-iam/src/applications/apicredentials/purpose"
	"github.com/a-digi/coco-iam/src/auth/crypto/bcrypt"
)

// fakeLookup is a CredentialLookup stub for tests. Zero-value fields
// mean "not found"; populate `cred` + `purposes` for a happy-path
// lookup.
type fakeLookup struct {
	cred     *entity.Credential
	purposes []string
	err      error
}

func (f fakeLookup) FindByAPIID(_ string) (*entity.Credential, []string, error) {
	if f.err != nil {
		return nil, nil, f.err
	}
	return f.cred, f.purposes, nil
}

// buildHeader base64-encodes a `user:pass` pair into an
// `Authorization: Basic ...` value.
func buildHeader(user, pass string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"+pass))
}

// validCredential returns a bcrypt-hashed credential that passes every
// check the authn layer applies, ready for the happy-path assertions.
// Cost 4 keeps the test fast while still exercising real bcrypt.
func validCredential(t *testing.T, secret string) *entity.Credential {
	t.Helper()
	hash, err := bcrypt.HashPassword(secret, 4)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	return &entity.Credential{
		ID:            "cred-1",
		ApplicationID: "app-1",
		APIID:         "api-1",
		SecretHash:    hash,
		IsActive:      true,
		ExpiresAt:     time.Now().Add(time.Hour),
	}
}

func TestAuthenticate_HappyPath(t *testing.T) {
	cred := validCredential(t, "s3cret")
	lookup := fakeLookup{cred: cred, purposes: []string{purpose.SecurityKeyRead.String()}}

	got, err := AuthenticateBasicAuth(
		buildHeader("api-1", "s3cret"),
		"app-1",
		purpose.SecurityKeyRead,
		time.Now(),
		lookup,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil || got.ID != "cred-1" {
		t.Errorf("want cred-1, got %+v", got)
	}
}

func TestAuthenticate_EmptyHeader(t *testing.T) {
	_, err := AuthenticateBasicAuth("", "app-1", purpose.SecurityKeyRead, time.Now(), fakeLookup{})
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("want ErrUnauthorized, got %v", err)
	}
}

func TestAuthenticate_WrongScheme(t *testing.T) {
	// A bearer token on the renew endpoint would be a confused
	// caller — reject it without even looking up the credential.
	_, err := AuthenticateBasicAuth(
		"Bearer abc.def.ghi",
		"app-1", purpose.SecurityKeyRead, time.Now(), fakeLookup{},
	)
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("want ErrUnauthorized, got %v", err)
	}
}

func TestAuthenticate_MalformedBase64(t *testing.T) {
	_, err := AuthenticateBasicAuth(
		"Basic !!!not-base64!!!",
		"app-1", purpose.SecurityKeyRead, time.Now(), fakeLookup{},
	)
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("want ErrUnauthorized, got %v", err)
	}
}

func TestAuthenticate_MissingColonInPayload(t *testing.T) {
	payload := base64.StdEncoding.EncodeToString([]byte("no-colon-here"))
	_, err := AuthenticateBasicAuth(
		"Basic "+payload,
		"app-1", purpose.SecurityKeyRead, time.Now(), fakeLookup{},
	)
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("want ErrUnauthorized, got %v", err)
	}
}

func TestAuthenticate_UnknownAPIID(t *testing.T) {
	// Repository signals "no row" either via ErrNotFound or by
	// returning (nil, nil, nil) in some code paths. Authn must
	// collapse both to unauthorized.
	lookup := fakeLookup{err: errors.New("not found")}
	_, err := AuthenticateBasicAuth(
		buildHeader("api-1", "s3cret"),
		"app-1", purpose.SecurityKeyRead, time.Now(), lookup,
	)
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("want ErrUnauthorized, got %v", err)
	}
}

func TestAuthenticate_WrongSecret(t *testing.T) {
	cred := validCredential(t, "correct-secret")
	lookup := fakeLookup{cred: cred, purposes: []string{purpose.SecurityKeyRead.String()}}

	_, err := AuthenticateBasicAuth(
		buildHeader("api-1", "WRONG"),
		"app-1",
		purpose.SecurityKeyRead,
		time.Now(),
		lookup,
	)
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("want ErrUnauthorized, got %v", err)
	}
}

func TestAuthenticate_RevokedCredential(t *testing.T) {
	cred := validCredential(t, "s3cret")
	cred.IsActive = false
	lookup := fakeLookup{cred: cred, purposes: []string{purpose.SecurityKeyRead.String()}}

	_, err := AuthenticateBasicAuth(
		buildHeader("api-1", "s3cret"),
		"app-1", purpose.SecurityKeyRead, time.Now(), lookup,
	)
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("revoked credential must be rejected, got %v", err)
	}
}

func TestAuthenticate_ExpiredCredential(t *testing.T) {
	cred := validCredential(t, "s3cret")
	cred.ExpiresAt = time.Now().Add(-time.Hour)
	lookup := fakeLookup{cred: cred, purposes: []string{purpose.SecurityKeyRead.String()}}

	_, err := AuthenticateBasicAuth(
		buildHeader("api-1", "s3cret"),
		"app-1", purpose.SecurityKeyRead, time.Now(), lookup,
	)
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("expired credential must be rejected, got %v", err)
	}
}

func TestAuthenticate_CrossTenantRejected(t *testing.T) {
	// Credential exists and matches the secret, but belongs to a
	// different application. Accepting this would let org A's
	// credential pull org B's keys — obvious but must be pinned.
	cred := validCredential(t, "s3cret")
	cred.ApplicationID = "app-OTHER"
	lookup := fakeLookup{cred: cred, purposes: []string{purpose.SecurityKeyRead.String()}}

	_, err := AuthenticateBasicAuth(
		buildHeader("api-1", "s3cret"),
		"app-1",
		purpose.SecurityKeyRead,
		time.Now(),
		lookup,
	)
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("cross-tenant credential must be rejected, got %v", err)
	}
}

func TestAuthenticate_MissingRequiredPurpose(t *testing.T) {
	// Credential is valid but doesn't carry the purpose this
	// endpoint requires. Handler must reject generically rather
	// than leak which purposes the credential does hold.
	cred := validCredential(t, "s3cret")
	lookup := fakeLookup{cred: cred, purposes: []string{"other:purpose"}}

	_, err := AuthenticateBasicAuth(
		buildHeader("api-1", "s3cret"),
		"app-1",
		purpose.SecurityKeyRead,
		time.Now(),
		lookup,
	)
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("purpose mismatch must be rejected, got %v", err)
	}
}

func TestAuthenticate_ExactExpiryIsRejected(t *testing.T) {
	// Boundary check: a credential whose expiry instant equals `now`
	// is expired (exclusive upper bound).
	cred := validCredential(t, "s3cret")
	expiry := time.Now().Add(time.Hour)
	cred.ExpiresAt = expiry
	lookup := fakeLookup{cred: cred, purposes: []string{purpose.SecurityKeyRead.String()}}

	_, err := AuthenticateBasicAuth(
		buildHeader("api-1", "s3cret"),
		"app-1",
		purpose.SecurityKeyRead,
		expiry,
		lookup,
	)
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("credential at exact expiry must be rejected, got %v", err)
	}
}

func TestAuthenticate_MultiplePurposesMatchRequired(t *testing.T) {
	// A credential that carries several purposes should pass the
	// check when any one of them matches the required purpose.
	cred := validCredential(t, "s3cret")
	lookup := fakeLookup{
		cred:     cred,
		purposes: []string{"other:purpose", purpose.SecurityKeyRead.String(), "more:stuff"},
	}

	_, err := AuthenticateBasicAuth(
		buildHeader("api-1", "s3cret"),
		"app-1",
		purpose.SecurityKeyRead,
		time.Now(),
		lookup,
	)
	if err != nil {
		t.Errorf("credential with matching purpose in a list should authenticate, got %v", err)
	}
}
