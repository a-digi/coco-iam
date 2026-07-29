package ipguard

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/a-digi/coco-server/server/di"
	"github.com/a-digi/coco-server/server/security"
	_ "github.com/mattn/go-sqlite3"

	"github.com/a-digi/coco-iam/src/security/dbhandle"
)

// freshDB opens an in-memory SQLite DB with the ip_bans/ip_allowlist
// schema — mirrors api/config/db/migrations/26_07_2026_10_00_00.sql.
func freshDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`
		CREATE TABLE ip_bans (
		    ip         TEXT NOT NULL CONSTRAINT ip_bans_pk PRIMARY KEY,
		    tier       TEXT NOT NULL,
		    reason     TEXT NOT NULL,
		    banned_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		    expires_at DATETIME NOT NULL,
		    hit_count  INTEGER NOT NULL DEFAULT 1,
		    created_by TEXT
		);
		CREATE TABLE ip_allowlist (
		    ip         TEXT NOT NULL CONSTRAINT ip_allowlist_pk PRIMARY KEY,
		    note       TEXT,
		    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		    created_by TEXT NOT NULL
		);
	`)
	if err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return db
}

// freshAttacksDB opens an in-memory SQLite DB with the ip_attacks/
// ip_attack_targets/db_meta schema — mirrors
// api/config/db/ip_attacks_migrations/001_initial.sql and 002_db_meta.sql.
func freshAttacksDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`
		CREATE TABLE ip_attacks (
		    id           TEXT NOT NULL CONSTRAINT ip_attacks_pk PRIMARY KEY,
		    ip           TEXT NOT NULL,
		    tier         TEXT NOT NULL,
		    started_at   DATETIME NOT NULL,
		    last_seen_at DATETIME NOT NULL,
		    ended_at     DATETIME,
		    hit_count    INTEGER NOT NULL DEFAULT 0,
		    ban_count    INTEGER NOT NULL DEFAULT 1,
		    origin_hint  TEXT,
		    geoip_info   TEXT
		);
		CREATE TABLE ip_attack_targets (
		    id          TEXT NOT NULL CONSTRAINT ip_attack_targets_pk PRIMARY KEY,
		    attack_id   TEXT NOT NULL CONSTRAINT ip_attack_targets_attack_fk REFERENCES ip_attacks (id),
		    path        TEXT NOT NULL,
		    method      TEXT NOT NULL,
		    hit_count   INTEGER NOT NULL DEFAULT 0,
		    body_sample TEXT
		);
		CREATE UNIQUE INDEX ip_attack_targets_unique_idx ON ip_attack_targets (attack_id, path, method);
		CREATE TABLE db_meta (key TEXT NOT NULL PRIMARY KEY, value TEXT NOT NULL);
		INSERT INTO db_meta (key, value) VALUES ('entry_count', '0');
	`)
	if err != nil {
		t.Fatalf("create attacks schema: %v", err)
	}
	return db
}

// freshAttacksHandle is freshAttacksDB wrapped in a *dbhandle.Handle —
// the type ipguard's constructors actually take. Use this at call
// sites that don't need direct SQL access to the attacks DB afterward;
// use mustAttacksHandle(t, freshAttacksDB(t)) when the raw *sql.DB is
// also needed for assertions.
func freshAttacksHandle(t *testing.T) *dbhandle.Handle {
	t.Helper()
	return mustAttacksHandle(t, freshAttacksDB(t))
}

func mustAttacksHandle(t *testing.T, db *sql.DB) *dbhandle.Handle {
	t.Helper()
	h, err := dbhandle.New(db)
	if err != nil {
		t.Fatalf("dbhandle.New() error = %v", err)
	}
	return h
}

// spyInner is a security.SecurityLayer that records how many times it
// was delegated to, standing in for the real ScopeSecurityLayer.
// calls is atomic since the concurrency test below calls Authorize
// from many goroutines at once.
type spyInner struct {
	calls int64
	err   error
}

func (s *spyInner) Authorize(w http.ResponseWriter, r *http.Request, ctx di.Context, route *security.Route) error {
	atomic.AddInt64(&s.calls, 1)
	return s.err
}

func testConfig() Config {
	return Config{
		TrustProxyIPHeaders: nil,
		RateLimit: RateLimitConfig{
			Global:         TierConfig{Requests: 3, WindowSeconds: 60, BanSeconds: 900},
			Sensitive:      TierConfig{Requests: 2, WindowSeconds: 300, BanSeconds: 3600},
			SensitivePaths: []string{"/admin/oauth/authenticate"},
		},
	}
}

func newTestGuard(t *testing.T, cfg Config) *IPGuardSecurityLayer {
	t.Helper()
	g, err := NewWithDB(cfg, &spyInner{}, freshDB(t), nil, freshAttacksHandle(t), nil, nil, nil)
	if err != nil {
		t.Fatalf("NewWithDB() error = %v", err)
	}
	return g
}

func request(ip, path string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, path, nil)
	r.RemoteAddr = ip + ":54321"
	return r
}

func TestAuthorize_AllowsWithinGlobalLimitThenBans(t *testing.T) {
	cfg := testConfig()
	inner := &spyInner{}
	g, err := NewWithDB(cfg, inner, freshDB(t), nil, freshAttacksHandle(t), nil, nil, nil)
	if err != nil {
		t.Fatalf("NewWithDB() error = %v", err)
	}

	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		if err := g.Authorize(w, request("203.0.113.7", "/anything"), nil, &security.Route{Path: "/anything"}); err != nil {
			t.Fatalf("call %d: Authorize() error = %v, want nil (within limit)", i+1, err)
		}
	}
	if inner.calls != 3 {
		t.Fatalf("inner.calls = %d, want 3", inner.calls)
	}

	w := httptest.NewRecorder()
	err = g.Authorize(w, request("203.0.113.7", "/anything"), nil, &security.Route{Path: "/anything"})
	if err == nil {
		t.Fatal("4th call: expected an error (rate limit exceeded)")
	}
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusTooManyRequests)
	}
	if w.Header().Get("Retry-After") == "" {
		t.Fatal("expected a Retry-After header on the 429 response")
	}
	if inner.calls != 3 {
		t.Fatalf("inner.calls after ban = %d, want still 3 (inner must not run once banned)", inner.calls)
	}
}

func TestAuthorize_BannedIPStaysBlockedOnSubsequentRequests(t *testing.T) {
	cfg := testConfig()
	inner := &spyInner{}
	g, err := NewWithDB(cfg, inner, freshDB(t), nil, freshAttacksHandle(t), nil, nil, nil)
	if err != nil {
		t.Fatalf("NewWithDB() error = %v", err)
	}

	for i := 0; i < 4; i++ {
		w := httptest.NewRecorder()
		_ = g.Authorize(w, request("203.0.113.7", "/anything"), nil, &security.Route{Path: "/anything"})
	}

	// A 5th request, even well under the raw counter's own window
	// limit, must be rejected purely because the IP is banned.
	w := httptest.NewRecorder()
	if err := g.Authorize(w, request("203.0.113.7", "/anything"), nil, &security.Route{Path: "/anything"}); err == nil {
		t.Fatal("expected banned IP to stay blocked")
	}
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusTooManyRequests)
	}
}

func TestAuthorize_SensitiveTierTripsBeforeGlobalOnGuessedLogins(t *testing.T) {
	cfg := testConfig() // sensitive limit = 2, global limit = 3
	inner := &spyInner{}
	g, err := NewWithDB(cfg, inner, freshDB(t), nil, freshAttacksHandle(t), nil, nil, nil)
	if err != nil {
		t.Fatalf("NewWithDB() error = %v", err)
	}

	route := &security.Route{Path: "/admin/oauth/authenticate"}
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		if err := g.Authorize(w, request("203.0.113.7", "/admin/oauth/authenticate"), nil, route); err != nil {
			t.Fatalf("call %d: Authorize() error = %v, want nil (within sensitive limit)", i+1, err)
		}
	}

	// 3rd call is still within the global limit (3) but exceeds the
	// stricter sensitive limit (2) — must be rejected.
	w := httptest.NewRecorder()
	err = g.Authorize(w, request("203.0.113.7", "/admin/oauth/authenticate"), nil, route)
	if err == nil {
		t.Fatal("expected the sensitive tier to trip before the global tier")
	}
	if inner.calls != 2 {
		t.Fatalf("inner.calls = %d, want 2", inner.calls)
	}
}

func TestAuthorize_NonSensitivePathIgnoresSensitiveTier(t *testing.T) {
	cfg := testConfig() // sensitive limit = 2, global limit = 3
	inner := &spyInner{}
	g, err := NewWithDB(cfg, inner, freshDB(t), nil, freshAttacksHandle(t), nil, nil, nil)
	if err != nil {
		t.Fatalf("NewWithDB() error = %v", err)
	}

	route := &security.Route{Path: "/dashboard/stats"} // not in SensitivePaths
	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		if err := g.Authorize(w, request("203.0.113.7", "/dashboard/stats"), nil, route); err != nil {
			t.Fatalf("call %d: Authorize() error = %v, want nil (global limit not yet hit)", i+1, err)
		}
	}
}

func TestAuthorize_AllowlistedIPBypassesEverything(t *testing.T) {
	cfg := testConfig()
	inner := &spyInner{}
	g, err := NewWithDB(cfg, inner, freshDB(t), nil, freshAttacksHandle(t), nil, nil, nil)
	if err != nil {
		t.Fatalf("NewWithDB() error = %v", err)
	}
	if err := g.AllowIP("203.0.113.7", "office egress", "admin-1"); err != nil {
		t.Fatalf("AllowIP() error = %v", err)
	}

	route := &security.Route{Path: "/admin/oauth/authenticate"}
	for i := 0; i < 10; i++ {
		w := httptest.NewRecorder()
		if err := g.Authorize(w, request("203.0.113.7", "/admin/oauth/authenticate"), nil, route); err != nil {
			t.Fatalf("call %d: allowlisted IP got blocked, error = %v", i+1, err)
		}
	}
	if inner.calls != 10 {
		t.Fatalf("inner.calls = %d, want 10 (never blocked)", inner.calls)
	}
}

func TestUnban_LiftsAnActiveBan(t *testing.T) {
	cfg := testConfig()
	g := newTestGuard(t, cfg)

	if err := g.Ban("203.0.113.7", "manual", "test", time.Hour, nil); err != nil {
		t.Fatalf("Ban() error = %v", err)
	}
	w := httptest.NewRecorder()
	if err := g.Authorize(w, request("203.0.113.7", "/x"), nil, &security.Route{Path: "/x"}); err == nil {
		t.Fatal("expected banned IP to be blocked before Unban")
	}

	if err := g.Unban("203.0.113.7"); err != nil {
		t.Fatalf("Unban() error = %v", err)
	}
	w = httptest.NewRecorder()
	if err := g.Authorize(w, request("203.0.113.7", "/x"), nil, &security.Route{Path: "/x"}); err != nil {
		t.Fatalf("expected request to succeed after Unban, error = %v", err)
	}
}

func TestUnban_ErrorsWhenIPNotBanned(t *testing.T) {
	g := newTestGuard(t, testConfig())
	if err := g.Unban("203.0.113.7"); err == nil {
		t.Fatal("expected an error unbanning an IP that was never banned")
	}
}

func TestBan_RecordsManualBanWithCreatedBy(t *testing.T) {
	g := newTestGuard(t, testConfig())
	adminID := "6b12ba0b-6b36-4a94-bce4-6ba3615b1f85"
	if err := g.Ban("203.0.113.7", "manual", "manually banned", time.Hour, &adminID); err != nil {
		t.Fatalf("Ban() error = %v", err)
	}
	bans, err := g.ListBans()
	if err != nil {
		t.Fatalf("ListBans() error = %v", err)
	}
	if len(bans) != 1 || bans[0].CreatedBy != adminID {
		t.Fatalf("ListBans() = %+v, want single manual ban created_by=%s", bans, adminID)
	}
}

func TestDisallowIP_RemovesAllowlistEntry(t *testing.T) {
	g := newTestGuard(t, testConfig())
	if err := g.AllowIP("203.0.113.7", "", "admin-1"); err != nil {
		t.Fatalf("AllowIP() error = %v", err)
	}
	if err := g.DisallowIP("203.0.113.7"); err != nil {
		t.Fatalf("DisallowIP() error = %v", err)
	}
	if g.isAllowed("203.0.113.7") {
		t.Fatal("IP should no longer be allowlisted after DisallowIP")
	}
}

func TestPruneExpiredBans_RemovesFromMemoryAndDB(t *testing.T) {
	g := newTestGuard(t, testConfig())

	if err := g.Ban("203.0.113.7", "global", "test", -time.Minute, nil); err != nil {
		t.Fatalf("Ban() error = %v", err)
	}
	if err := g.PruneExpiredBans(); err != nil {
		t.Fatalf("PruneExpiredBans() error = %v", err)
	}

	w := httptest.NewRecorder()
	if err := g.Authorize(w, request("203.0.113.7", "/x"), nil, &security.Route{Path: "/x"}); err != nil {
		t.Fatalf("expired-then-pruned ban should no longer block, error = %v", err)
	}
	bans, err := g.ListBans()
	if err != nil {
		t.Fatalf("ListBans() error = %v", err)
	}
	if len(bans) != 0 {
		t.Fatalf("ListBans() after prune = %+v, want empty", bans)
	}
}

func TestHydrate_LoadsActiveBansAndAllowlistFromDB(t *testing.T) {
	cfg := testConfig()
	db := freshDB(t)

	// Seed rows directly, as if a prior process instance had written
	// them before this one started.
	if _, err := db.Exec(
		`INSERT INTO ip_bans (ip, tier, reason, expires_at) VALUES (?, ?, ?, ?)`,
		"203.0.113.7", "global", "seeded", time.Now().Add(time.Hour).UTC().Format(timeLayout),
	); err != nil {
		t.Fatalf("seed ip_bans: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO ip_allowlist (ip, created_by) VALUES (?, ?)`,
		"9.9.9.9", "admin-1",
	); err != nil {
		t.Fatalf("seed ip_allowlist: %v", err)
	}

	g, err := NewWithDB(cfg, &spyInner{}, db, nil, freshAttacksHandle(t), nil, nil, nil)
	if err != nil {
		t.Fatalf("NewWithDB() error = %v", err)
	}

	w := httptest.NewRecorder()
	if err := g.Authorize(w, request("203.0.113.7", "/x"), nil, &security.Route{Path: "/x"}); err == nil {
		t.Fatal("expected the pre-seeded ban to be hydrated and enforced immediately")
	}
	if !g.isAllowed("9.9.9.9") {
		t.Fatal("expected the pre-seeded allowlist entry to be hydrated")
	}
}

func TestHydrate_DoesNotLoadAlreadyExpiredBans(t *testing.T) {
	cfg := testConfig()
	db := freshDB(t)
	if _, err := db.Exec(
		`INSERT INTO ip_bans (ip, tier, reason, expires_at) VALUES (?, ?, ?, ?)`,
		"203.0.113.7", "global", "long expired", time.Now().Add(-time.Hour).UTC().Format(timeLayout),
	); err != nil {
		t.Fatalf("seed ip_bans: %v", err)
	}

	g, err := NewWithDB(cfg, &spyInner{}, db, nil, freshAttacksHandle(t), nil, nil, nil)
	if err != nil {
		t.Fatalf("NewWithDB() error = %v", err)
	}
	w := httptest.NewRecorder()
	if err := g.Authorize(w, request("203.0.113.7", "/x"), nil, &security.Route{Path: "/x"}); err != nil {
		t.Fatalf("an already-expired ban must not be hydrated as active, error = %v", err)
	}
}

// TestAuthorize_ConcurrentRequestsFromSameIPAreCountedExactly exercises
// the actual hot path under real parallel load (run with -race) — the
// production Authorize call is reached concurrently by many in-flight
// requests, so the limiter/ban maps must not lose or double-count
// hits under contention.
func TestAuthorize_ConcurrentRequestsFromSameIPAreCountedExactly(t *testing.T) {
	cfg := Config{
		TrustProxyIPHeaders: nil,
		RateLimit: RateLimitConfig{
			Global:    TierConfig{Requests: 50, WindowSeconds: 60, BanSeconds: 900},
			Sensitive: TierConfig{Requests: 1000, WindowSeconds: 300, BanSeconds: 3600},
		},
	}
	g := newTestGuard(t, cfg)
	route := &security.Route{Path: "/x"}

	const workers = 200
	var wg sync.WaitGroup
	var allowed, blocked int64
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			w := httptest.NewRecorder()
			if err := g.Authorize(w, request("203.0.113.7", "/x"), nil, route); err != nil {
				atomic.AddInt64(&blocked, 1)
			} else {
				atomic.AddInt64(&allowed, 1)
			}
		}()
	}
	wg.Wait()

	if allowed != 50 {
		t.Fatalf("allowed = %d, want exactly 50 (the configured limit)", allowed)
	}
	if blocked != workers-50 {
		t.Fatalf("blocked = %d, want %d", blocked, workers-50)
	}
}
