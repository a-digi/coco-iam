// Package authstate wraps per-org oauth_auth_requests table.
// One row per in-flight authorize handshake; rows carry the CSRF
// state parameter, the PKCE code_verifier, the IdP provider key,
// the caller's return_url, and a creation timestamp.
//
// Lifecycle:
//   - authorize handler: StartAuthRequest → row persisted in org DB.
//   - callback handler: ConsumeAuthRequest → row fetched + deleted
//     from org DB.
//
// TTL is 10 minutes by default; callers pass time.Now-backed context
// so tests can run deterministically.
//
// Routing: ConsumeAuthRequest scans all known per-org oauth DBs via
// WithKnownOrgIDs to find the org that holds the state row.
package authstate

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"time"
)

// DefaultTTL bounds how long an authorize handshake may remain
// open before the state row is considered stale. Long enough for
// slow IdP flows, short enough to bound the CSRF attack window.
const DefaultTTL = 10 * time.Minute

// ErrStateNotFound is returned by ConsumeAuthRequest when the
// state key doesn't exist OR has TTL-expired. The handler maps
// both to the same user-facing 400 so an attacker can't tell
// the two apart via timing.
var ErrStateNotFound = errors.New("authstate: state not found or expired")

// Store is the persistent facade over oauth_auth_requests.
type Store struct {
	mainDB      *sql.DB                            // sole DB in single-DB mode
	resolve     func(orgID string) (*sql.DB, error)
	knownOrgIDs func() []string                    // enumerates all org IDs for scan routing
	appResolve  func(appID string) (string, error) // maps appID to orgID
	ttl         time.Duration
	now         func() time.Time
}

// New constructs a Store.
//
// mainDB is used only in single-DB mode (resolve == nil).
//
// resolve is a function that returns the per-org oauth.db for a given
// org ID. When nil, mainDB is used for token storage (single-DB legacy
// / test mode).
//
// ttl <= 0 picks DefaultTTL. nowFn nil picks time.Now so production
// callers can just pass nil.
func New(mainDB *sql.DB, resolve func(orgID string) (*sql.DB, error), ttl time.Duration, nowFn func() time.Time) *Store {
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	if nowFn == nil {
		nowFn = time.Now
	}
	return &Store{mainDB: mainDB, resolve: resolve, ttl: ttl, now: nowFn}
}

// WithAppResolver returns a copy of the Store that uses appResolve to
// map application IDs to org IDs.
func (s *Store) WithAppResolver(appResolve func(appID string) (string, error)) *Store {
	c := *s
	c.appResolve = appResolve
	return &c
}

// WithKnownOrgIDs returns a copy of the Store that uses fn to enumerate
// all org IDs for scan-based routing in multi-DB mode.
func (s *Store) WithKnownOrgIDs(fn func() []string) *Store {
	c := *s
	c.knownOrgIDs = fn
	return &c
}


// AuthRequest carries one row's contents.
type AuthRequest struct {
	State         string
	ApplicationID string
	Provider      string
	CodeVerifier  string
	ReturnURL     string
	CreatedAt     time.Time
}

// StartInput is the payload StartAuthRequest persists.
type StartInput struct {
	ApplicationID string
	Provider      string
	ReturnURL     string
}

// StartAuthRequest generates a random state + PKCE verifier,
// persists them in the org DB, writes a routing entry to mainDB,
// and returns the full handshake material the authorize handler
// needs to redirect the user. Callers should never modify
// CodeVerifier after receiving it; it stays server-side for the
// callback exchange.
func (s *Store) StartAuthRequest(in StartInput) (AuthRequest, string, error) {
	state, err := randomBase64URL(32)
	if err != nil {
		return AuthRequest{}, "", fmt.Errorf("authstate: state rand: %w", err)
	}
	verifier, err := randomBase64URL(32)
	if err != nil {
		return AuthRequest{}, "", fmt.Errorf("authstate: verifier rand: %w", err)
	}
	now := s.now().UTC()

	tokenDB, _, err := s.tokenDB(in.ApplicationID)
	if err != nil {
		return AuthRequest{}, "", fmt.Errorf("authstate: resolve org db: %w", err)
	}

	_, err = tokenDB.Exec(
		`INSERT INTO oauth_auth_requests
		 (state, application_id, provider, code_verifier, return_url, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		state, in.ApplicationID, in.Provider, verifier, in.ReturnURL,
		now.Format(time.RFC3339Nano),
	)
	if err != nil {
		return AuthRequest{}, "", fmt.Errorf("authstate: insert: %w", err)
	}

	req := AuthRequest{
		State:         state,
		ApplicationID: in.ApplicationID,
		Provider:      in.Provider,
		CodeVerifier:  verifier,
		ReturnURL:     in.ReturnURL,
		CreatedAt:     now,
	}
	return req, CodeChallenge(verifier), nil
}

// ConsumeAuthRequest fetches the row by state from the org DB,
// deletes it (so the state is single-use even under concurrent
// callbacks), and returns the row for the handler to verify. Rows
// older than TTL behave as if they don't exist.
func (s *Store) ConsumeAuthRequest(state string) (*AuthRequest, error) {
	tokenDB, err := s.tokenDBByState(state)
	if err != nil {
		return nil, err
	}

	tx, err := tokenDB.Begin()
	if err != nil {
		return nil, fmt.Errorf("authstate: begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	row := tx.QueryRow(
		`SELECT state, application_id, provider, code_verifier, return_url, created_at
		 FROM oauth_auth_requests WHERE state = ? LIMIT 1`, state,
	)
	var req AuthRequest
	var createdRaw string
	if err := row.Scan(
		&req.State, &req.ApplicationID, &req.Provider, &req.CodeVerifier,
		&req.ReturnURL, &createdRaw,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrStateNotFound
		}
		return nil, fmt.Errorf("authstate: scan: %w", err)
	}
	created, err := parseTime(createdRaw)
	if err != nil {
		return nil, fmt.Errorf("authstate: parse created_at: %w", err)
	}
	req.CreatedAt = created

	if s.now().Sub(created) > s.ttl {
		// Expired — remove and treat as missing.
		_, _ = tx.Exec(`DELETE FROM oauth_auth_requests WHERE state = ?`, state)
		if cerr := tx.Commit(); cerr != nil {
			return nil, fmt.Errorf("authstate: commit expired delete: %w", cerr)
		}
		return nil, ErrStateNotFound
	}

	if _, err := tx.Exec(`DELETE FROM oauth_auth_requests WHERE state = ?`, state); err != nil {
		return nil, fmt.Errorf("authstate: delete: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("authstate: commit: %w", err)
	}

	return &req, nil
}

// -- internal routing helpers ------------------------------------------

// tokenDB returns the DB that should store tokens for the given
// applicationID, plus the resolved orgID (empty in single-DB mode).
func (s *Store) tokenDB(applicationID string) (*sql.DB, string, error) {
	if s.resolve == nil {
		return s.mainDB, "", nil
	}
	var orgID string
	if s.appResolve != nil {
		var err error
		orgID, err = s.appResolve(applicationID)
		if err != nil {
			return nil, "", fmt.Errorf("authstate: lookup org for app %s: %w", applicationID, err)
		}
	} else if err := s.mainDB.QueryRow(
		`SELECT org_id FROM application_org_index WHERE application_id = ? LIMIT 1`,
		applicationID,
	).Scan(&orgID); err != nil {
		return nil, "", fmt.Errorf("authstate: lookup org for app %s: %w", applicationID, err)
	}
	db, err := s.resolve(orgID)
	if err != nil {
		return nil, "", fmt.Errorf("authstate: open org db %s: %w", orgID, err)
	}
	return db, orgID, nil
}

// tokenDBByState returns the DB that holds the given state token by
// scanning all known per-org oauth DBs.
func (s *Store) tokenDBByState(state string) (*sql.DB, error) {
	if s.resolve == nil {
		return s.mainDB, nil
	}
	if s.knownOrgIDs == nil {
		return nil, ErrStateNotFound
	}
	for _, orgID := range s.knownOrgIDs() {
		db, err := s.resolve(orgID)
		if err != nil || db == nil {
			continue
		}
		var found string
		if db.QueryRow(
			`SELECT state FROM oauth_auth_requests WHERE state = ? LIMIT 1`, state,
		).Scan(&found) == nil {
			return db, nil
		}
	}
	return nil, ErrStateNotFound
}

// -- helpers -------------------------------------------------------

// CodeChallenge returns the RFC 7636 S256 challenge for the
// given verifier (base64-url no-pad).
func CodeChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func randomBase64URL(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func parseTime(s string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("unparseable timestamp %q", s)
}
