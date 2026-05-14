package keys

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"time"
)

// Service is the façade the rest of the codebase consumes. It wires
// the DB-backed metadata Store with the on-disk FileStore and owns
// the lifecycle rules: at most one active + one pending per app,
// deactivated keys keep verifying for 24 hours, files never get
// deleted.
type Service struct {
	Rows  *Store
	Files *FileStore
}

func NewService(rows *Store, files *FileStore) *Service {
	return &Service{Rows: rows, Files: files}
}

// -- lifecycle ---------------------------------------------------------

// EnsureActive is called from the application-create listener. If
// an application has no active key on record, we generate one and
// stamp it as active. Idempotent: repeated calls after the active
// key exists are no-ops.
func (s *Service) EnsureActive(appID string) error {
	if appID == "" {
		return fmt.Errorf("keys: application id is required")
	}
	if _, err := s.Rows.Active(appID); err == nil {
		return nil
	} else if !errors.Is(err, ErrNotFound) {
		return err
	}
	_, err := s.generateWithStatus(appID, StatusActive, time.Now().UTC(), nil)
	return err
}

// GeneratePending creates a fresh keypair and persists it in the
// `pending` state. Refuses with ErrPendingExists when one is already
// present — admins must discard or accept the existing pending key
// first.
func (s *Service) GeneratePending(appID string) (KeyRow, error) {
	if _, err := s.Rows.Pending(appID); err == nil {
		return KeyRow{}, ErrPendingExists
	} else if !errors.Is(err, ErrNotFound) {
		return KeyRow{}, err
	}
	return s.generateWithStatus(appID, StatusPending, time.Time{}, nil)
}

// ActivatePending promotes the pending key to active and demotes the
// former active to deactivated with a 24-hour expiry. We do this as
// two sequential updates — SQLite's implicit transaction is enough
// for the window this actually runs in (one admin click); if both
// updates succeed we're consistent, if the first fails the pending
// key stays pending, if the second fails the system notices one
// app with zero active keys and EnsureActive on next use fixes it.
func (s *Service) ActivatePending(appID string) error {
	pending, err := s.Rows.Pending(appID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrNoPending
		}
		return err
	}
	now := time.Now().UTC()

	// Demote the current active (if any) — first-time apps may not
	// have one, though the PostEventListener ensures they do.
	if current, aerr := s.Rows.Active(appID); aerr == nil {
		expiresAt := now.Add(DeactivatedGrace)
		if err := s.Rows.UpdateStatus(appID, current.ID, StatusDeactivated, current.ActivatedAt, &now, &expiresAt); err != nil {
			return err
		}
	} else if !errors.Is(aerr, ErrNotFound) {
		return aerr
	}

	// Promote pending → active.
	if err := s.Rows.UpdateStatus(appID, pending.ID, StatusActive, &now, nil, nil); err != nil {
		return err
	}
	return nil
}

// DiscardPending removes the pending row + its PEM files. This is
// the ONE path that deletes files from disk — per the spec active
// and deactivated keys stay forever.
func (s *Service) DiscardPending(appID string) error {
	pending, err := s.Rows.Pending(appID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrNoPending
		}
		return err
	}
	if err := s.Rows.Delete(appID, pending.ID); err != nil {
		return err
	}
	// File cleanup is best-effort — the row is already gone, so
	// even if the folder remains it has no DB reference and won't be
	// loaded.
	_ = s.Files.Delete(appID, pending.ID)
	return nil
}

// DeactivateCompletely forces `expires_at = now` on a deactivated
// key. The row and PEM stay on disk for audit; verification just
// fails going forward. Only keys already in the `deactivated`
// status may be targeted — force-expiring the active key would
// orphan the application, and there's no reason to expire a
// pending one.
func (s *Service) DeactivateCompletely(appID, keyID string) error {
	row, err := s.Rows.Get(appID, keyID)
	if err != nil {
		return err
	}
	if row.Status != StatusDeactivated {
		return ErrNotDeactivated
	}
	now := time.Now().UTC()
	return s.Rows.UpdateStatus(appID, row.ID, StatusDeactivated, row.ActivatedAt, row.DeactivatedAt, &now)
}

// -- accessors ---------------------------------------------------------

// List returns every non-expired key row for an application, newest
// first. UI consumes this to render the three sections (active,
// pending, deactivated).
func (s *Service) List(appID string) ([]KeyRow, error) {
	rows, err := s.Rows.List(appID)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	out := rows[:0]
	for _, r := range rows {
		// Expired deactivated rows stay in the DB but don't show up
		// in the UI either — they're dead weight for the admin.
		if r.Status == StatusDeactivated && r.ExpiresAt != nil && !r.ExpiresAt.After(now) {
			continue
		}
		out = append(out, r)
	}
	return out, nil
}

// Keypair wraps the on-disk PEMs with the row metadata so the admin
// UI gets everything it needs in one call.
func (s *Service) Keypair(appID, keyID string, includePrivate bool) (Keypair, error) {
	row, err := s.Rows.Get(appID, keyID)
	if err != nil {
		return Keypair{}, err
	}
	return s.keypairForRow(row, includePrivate)
}

func (s *Service) keypairForRow(row KeyRow, includePrivate bool) (Keypair, error) {
	pubPEM, err := s.Files.ReadPublicPEM(row.ApplicationID, row.ID)
	if err != nil {
		return Keypair{}, err
	}
	out := Keypair{
		ID:            row.ID,
		ApplicationID: row.ApplicationID,
		Status:        row.Status,
		PublicPEM:     string(pubPEM),
		HasPrivate:    true,
		CreatedAt:     row.CreatedAt,
		ActivatedAt:   row.ActivatedAt,
		DeactivatedAt: row.DeactivatedAt,
		ExpiresAt:     row.ExpiresAt,
	}
	if includePrivate {
		privPEM, err := s.Files.ReadPrivatePEM(row.ApplicationID, row.ID)
		if err != nil && !errors.Is(err, ErrNotFound) {
			return Keypair{}, err
		}
		out.PrivatePEM = string(privPEM)
	}
	return out, nil
}

// Keypairs returns the per-row Keypair for every non-expired key of
// the application. `includePrivate` applies to all rows uniformly.
func (s *Service) Keypairs(appID string, includePrivate bool) ([]Keypair, error) {
	rows, err := s.List(appID)
	if err != nil {
		return nil, err
	}
	out := make([]Keypair, 0, len(rows))
	for _, r := range rows {
		kp, kerr := s.keypairForRow(r, includePrivate)
		if kerr != nil {
			return nil, kerr
		}
		out = append(out, kp)
	}
	return out, nil
}

// ActiveRow / LoadActivePrivateKey together give the signing path
// what it needs: a private key and the kid that generated it.
func (s *Service) ActiveRow(appID string) (KeyRow, error) {
	return s.Rows.Active(appID)
}

func (s *Service) LoadPrivateKey(appID, kid string) (*rsa.PrivateKey, error) {
	data, err := s.Files.ReadPrivatePEM(appID, kid)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("keys: private.pem is not PEM-encoded")
	}
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("keys: private.pem is not an RSA key")
		}
		return rsaKey, nil
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	return nil, fmt.Errorf("keys: private.pem is not a recognised RSA key")
}

// LoadVerifiablePublicKey loads the public key for one kid, but only
// if that kid is still in a verifiable state. Used by the renew
// handler so that a refresh token signed by a key whose
// `expires_at` has passed is rejected even though the PEM file is
// still on disk.
func (s *Service) LoadVerifiablePublicKey(appID, kid string) (*rsa.PublicKey, error) {
	row, err := s.Rows.Get(appID, kid)
	if err != nil {
		return nil, err
	}
	if row.ApplicationID != appID {
		return nil, ErrKeyNotVerifiable
	}
	if !row.IsVerifiable(time.Now().UTC()) {
		return nil, ErrKeyNotVerifiable
	}
	return s.loadPublicKey(appID, kid)
}

func (s *Service) loadPublicKey(appID, kid string) (*rsa.PublicKey, error) {
	data, err := s.Files.ReadPublicPEM(appID, kid)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("keys: public.pem is not PEM-encoded")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("keys: parse public.pem: %w", err)
	}
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("keys: public.pem is not an RSA key")
	}
	return rsaPub, nil
}

// VerifiableJWKS returns the JWKS array for an application — one
// entry per key that still validates tokens (active + deactivated
// with unexpired `expires_at`). Downstream services call
// `/public/applications/:id/.well-known/jwks.json` which wraps this
// in `{ keys: [...] }`.
func (s *Service) VerifiableJWKS(appID string) ([]map[string]any, error) {
	rows, err := s.Rows.List(appID)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		if !r.IsVerifiable(now) {
			continue
		}
		pub, err := s.loadPublicKey(appID, r.ID)
		if err != nil {
			// A row without a file is a data anomaly; skip rather
			// than failing the whole JWKS response and breaking
			// every caller.
			continue
		}
		out = append(out, map[string]any{
			"kty": "RSA",
			"alg": "RS256",
			"use": "sig",
			"kid": r.ID,
			"n":   base64urlUint(pub.N),
			"e":   base64urlUint(big.NewInt(int64(pub.E))),
		})
	}
	return out, nil
}

// -- internals ---------------------------------------------------------

// generateWithStatus creates the PEM pair, writes it to disk, then
// inserts the row. If the DB insert fails we leave the files in place
// (no unique-ID clash is possible — every call uses a fresh UUID), and
// return the error.
func (s *Service) generateWithStatus(appID string, status KeyStatus, activatedAt time.Time, expiresAt *time.Time) (KeyRow, error) {
	if appID == "" {
		return KeyRow{}, fmt.Errorf("keys: application id is required")
	}
	priv, err := rsa.GenerateKey(rand.Reader, KeySize)
	if err != nil {
		return KeyRow{}, fmt.Errorf("keys: generate rsa: %w", err)
	}
	privPEM, err := marshalPrivatePEM(priv)
	if err != nil {
		return KeyRow{}, err
	}
	pubPEM, err := marshalPublicPEM(&priv.PublicKey)
	if err != nil {
		return KeyRow{}, err
	}
	kid := newKID()
	if err := s.Files.Write(appID, kid, privPEM, pubPEM); err != nil {
		return KeyRow{}, err
	}
	row := KeyRow{
		ID:            kid,
		ApplicationID: appID,
		Status:        status,
		CreatedAt:     time.Now().UTC(),
	}
	if !activatedAt.IsZero() {
		ts := activatedAt
		row.ActivatedAt = &ts
	}
	row.ExpiresAt = expiresAt
	if err := s.Rows.Insert(row); err != nil {
		return KeyRow{}, err
	}
	return row, nil
}

// -- PEM helpers -------------------------------------------------------

func marshalPrivatePEM(priv *rsa.PrivateKey) ([]byte, error) {
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, fmt.Errorf("keys: marshal private: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), nil
}

func marshalPublicPEM(pub *rsa.PublicKey) ([]byte, error) {
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return nil, fmt.Errorf("keys: marshal public: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}), nil
}

func base64urlUint(i *big.Int) string {
	return base64.RawURLEncoding.EncodeToString(i.Bytes())
}

// newKID is a random UUIDv4-shaped identifier. We don't pull in
// google/uuid because the rest of the codebase uses this same
// 16-bytes-of-crypto-rand pattern (see loginpage/assets.go).
func newKID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "00000000-0000-0000-0000-000000000000"
	}
	hx := hex.EncodeToString(buf)
	return fmt.Sprintf("%s-%s-%s-%s-%s", hx[:8], hx[8:12], hx[12:16], hx[16:20], hx[20:32])
}
