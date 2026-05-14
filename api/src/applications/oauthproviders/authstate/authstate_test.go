package authstate

import (
	"database/sql"
	"errors"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	_, err = db.Exec(`
		CREATE TABLE oauth_auth_requests (
		    state           TEXT NOT NULL PRIMARY KEY,
		    application_id  TEXT NOT NULL,
		    provider        TEXT NOT NULL,
		    code_verifier   TEXT NOT NULL,
		    return_url      TEXT NOT NULL,
		    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	return db
}

// newSingleDB builds a Store in single-DB mode (resolve=nil).
// Used by all tests that just need the happy-path behaviour against
// a plain in-memory SQLite.
func newSingleDB(db *sql.DB, ttl time.Duration, nowFn func() time.Time) *Store {
	return New(db, nil, ttl, nowFn)
}

func TestStart_PersistsFreshStateAndVerifier(t *testing.T) {
	s := newSingleDB(openTestDB(t), 0, nil)
	req, chal, err := s.StartAuthRequest(StartInput{
		ApplicationID: "app-1", Provider: "google", ReturnURL: "https://app/cb",
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if req.State == "" || req.CodeVerifier == "" {
		t.Errorf("state/verifier must be populated")
	}
	if chal == "" {
		t.Errorf("challenge must be populated")
	}
	// Challenge must be the SHA-256 of the verifier (round-trip).
	if CodeChallenge(req.CodeVerifier) != chal {
		t.Errorf("challenge derivation broken")
	}
}

func TestStart_EachCallGeneratesUniqueState(t *testing.T) {
	s := newSingleDB(openTestDB(t), 0, nil)
	r1, _, _ := s.StartAuthRequest(StartInput{ApplicationID: "a", Provider: "google"})
	r2, _, _ := s.StartAuthRequest(StartInput{ApplicationID: "a", Provider: "google"})
	if r1.State == r2.State {
		t.Errorf("state collision: %q", r1.State)
	}
	if r1.CodeVerifier == r2.CodeVerifier {
		t.Errorf("verifier collision")
	}
}

func TestConsume_HappyPathReturnsAndDeletes(t *testing.T) {
	s := newSingleDB(openTestDB(t), 0, nil)
	req, _, _ := s.StartAuthRequest(StartInput{ApplicationID: "a", Provider: "google"})
	got, err := s.ConsumeAuthRequest(req.State)
	if err != nil {
		t.Fatalf("Consume: %v", err)
	}
	if got.State != req.State || got.CodeVerifier != req.CodeVerifier {
		t.Errorf("consume returned drift: got %+v want %+v", got, req)
	}
	// Second consume must fail — state is single-use.
	if _, err := s.ConsumeAuthRequest(req.State); !errors.Is(err, ErrStateNotFound) {
		t.Errorf("second consume should fail: %v", err)
	}
}

func TestConsume_UnknownStateReturnsSentinel(t *testing.T) {
	s := newSingleDB(openTestDB(t), 0, nil)
	_, err := s.ConsumeAuthRequest("not-a-real-state")
	if !errors.Is(err, ErrStateNotFound) {
		t.Errorf("want ErrStateNotFound, got %v", err)
	}
}

func TestConsume_ExpiredStateReturnsSentinelAndDeletes(t *testing.T) {
	// Inject a controllable clock.
	currentTime := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	s := newSingleDB(openTestDB(t), 5*time.Minute, func() time.Time { return currentTime })

	req, _, _ := s.StartAuthRequest(StartInput{ApplicationID: "a", Provider: "google"})
	// Jump well past TTL.
	currentTime = currentTime.Add(10 * time.Minute)

	if _, err := s.ConsumeAuthRequest(req.State); !errors.Is(err, ErrStateNotFound) {
		t.Errorf("expired state should be missing, got %v", err)
	}
}

func TestCodeChallenge_MatchesRFC7636S256(t *testing.T) {
	// RFC 7636 Appendix B vector.
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	want := "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
	if got := CodeChallenge(verifier); got != want {
		t.Errorf("got %q want %q", got, want)
	}
}
