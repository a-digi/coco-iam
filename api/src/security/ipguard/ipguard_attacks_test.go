package ipguard

import (
	"context"
	"database/sql"
	"fmt"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/a-digi/coco-logger/logger"
	"github.com/a-digi/coco-server/server/security"
)

// spyLogger captures Warning() calls (what recordAttackHit writes to
// the dedicated attack log) without touching the filesystem.
type spyLogger struct {
	mu       sync.Mutex
	warnings []string
}

func (s *spyLogger) Info(msg string, args ...interface{})  {}
func (s *spyLogger) Error(msg string, args ...interface{}) {}
func (s *spyLogger) Close()                                {}
func (s *spyLogger) Warning(msg string, args ...interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.warnings = append(s.warnings, fmt.Sprintf(msg, args...))
}

func (s *spyLogger) messages() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.warnings))
	copy(out, s.warnings)
	return out
}

func newTestGuardWithAttacks(t *testing.T, cfg Config, attackLog *spyLogger) (*IPGuardSecurityLayer, *sql.DB) {
	t.Helper()
	attacksDB := freshAttacksDB(t)
	var l logger.Logger
	if attackLog != nil {
		l = attackLog
	}
	g, err := NewWithDB(cfg, &spyInner{}, freshDB(t), nil, mustAttacksHandle(t, attacksDB), l, nil)
	if err != nil {
		t.Fatalf("NewWithDB() error = %v", err)
	}
	return g, attacksDB
}

func countAttacks(t *testing.T, db *sql.DB, ip string) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM ip_attacks WHERE ip = ?`, ip).Scan(&n); err != nil {
		t.Fatalf("count attacks: %v", err)
	}
	return n
}

func TestRecordAttackHit_OneEpisodePerIPAcrossManyRejections(t *testing.T) {
	cfg := testConfig() // global limit 3
	log := &spyLogger{}
	g, attacksDB := newTestGuardWithAttacks(t, cfg, log)

	route := &security.Route{Path: "/anything"}
	for i := 0; i < 8; i++ {
		w := httptest.NewRecorder()
		_ = g.Authorize(w, request("203.0.113.7", "/anything"), nil, route)
	}

	if got := countAttacks(t, attacksDB, "203.0.113.7"); got != 1 {
		t.Fatalf("ip_attacks rows for the IP = %d, want exactly 1 (one episode, not one per rejected request)", got)
	}

	if len(log.messages()) == 0 {
		t.Fatal("expected at least one line written to the dedicated attack log")
	}
}

func TestRecordAttackHit_TargetsAggregatePerEndpoint(t *testing.T) {
	cfg := testConfig() // global limit 3: calls 1-3 allowed, call 4+ rejected
	g, attacksDB := newTestGuardWithAttacks(t, cfg, nil)

	// 5 calls to /a: 3 allowed, then call 4 trips the ban (rejected,
	// recorded) and call 5 is rejected as already-banned (recorded) —
	// 2 rejections against /a.
	for i := 0; i < 5; i++ {
		w := httptest.NewRecorder()
		_ = g.Authorize(w, request("203.0.113.7", "/a"), nil, &security.Route{Path: "/a"})
	}
	// Now already banned: every call to /b is rejected and recorded.
	for i := 0; i < 5; i++ {
		w := httptest.NewRecorder()
		_ = g.Authorize(w, request("203.0.113.7", "/b"), nil, &security.Route{Path: "/b"})
	}

	g.FlushAttacks()

	rows, err := attacksDB.Query(`SELECT path, hit_count FROM ip_attack_targets ORDER BY path`)
	if err != nil {
		t.Fatalf("query targets: %v", err)
	}
	defer rows.Close()

	counts := map[string]int{}
	for rows.Next() {
		var path string
		var hitCount int
		if err := rows.Scan(&path, &hitCount); err != nil {
			t.Fatalf("scan target: %v", err)
		}
		counts[path] = hitCount
	}
	if counts["/a"] != 2 {
		t.Fatalf("hit count for /a = %d, want 2 (the trip + one already-banned hit)", counts["/a"])
	}
	if counts["/b"] != 5 {
		t.Fatalf("hit count for /b = %d, want 5 (all rejected, already banned)", counts["/b"])
	}
}

func TestFlushAttacks_WritesCurrentTotals(t *testing.T) {
	cfg := testConfig() // global limit 3: calls 1-3 allowed, call 4+ rejected
	g, attacksDB := newTestGuardWithAttacks(t, cfg, nil)

	for i := 0; i < 6; i++ {
		w := httptest.NewRecorder()
		_ = g.Authorize(w, request("203.0.113.7", "/a"), nil, &security.Route{Path: "/a"})
	}
	g.FlushAttacks()

	var hitCount, banCount int
	var endedAt sql.NullString
	err := attacksDB.QueryRow(`SELECT hit_count, ban_count, ended_at FROM ip_attacks WHERE ip = ?`, "203.0.113.7").
		Scan(&hitCount, &banCount, &endedAt)
	if err != nil {
		t.Fatalf("query attack row: %v", err)
	}
	if hitCount != 3 { // calls 4, 5, 6 out of 6 total are the rejections
		t.Fatalf("hit_count = %d, want 3", hitCount)
	}
	if banCount != 1 {
		t.Fatalf("ban_count = %d, want 1 (tripped once)", banCount)
	}
	if endedAt.Valid {
		t.Fatal("episode should still be open (recent activity, grace period not elapsed)")
	}
}

func TestFlushAttacks_ClosesEpisodeAfterGracePeriod(t *testing.T) {
	cfg := Config{
		TrustProxyIPHeader: "",
		RateLimit: RateLimitConfig{
			Global:    TierConfig{Requests: 1, WindowSeconds: 60, BanSeconds: 1}, // grace = 2s
			Sensitive: TierConfig{Requests: 1000, WindowSeconds: 300, BanSeconds: 3600},
		},
	}
	g, attacksDB := newTestGuardWithAttacks(t, cfg, nil)

	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		_ = g.Authorize(w, request("203.0.113.7", "/a"), nil, &security.Route{Path: "/a"})
	}

	time.Sleep(2100 * time.Millisecond) // past the 2x ban_seconds grace period
	g.FlushAttacks()

	var endedAt sql.NullString
	if err := attacksDB.QueryRow(`SELECT ended_at FROM ip_attacks WHERE ip = ?`, "203.0.113.7").Scan(&endedAt); err != nil {
		t.Fatalf("query attack row: %v", err)
	}
	if !endedAt.Valid {
		t.Fatal("episode should be closed after its grace period elapsed with no new activity")
	}
}

func TestRecordAttackHit_NewEpisodeAfterPriorOneClosed(t *testing.T) {
	cfg := Config{
		TrustProxyIPHeader: "",
		RateLimit: RateLimitConfig{
			Global:    TierConfig{Requests: 1, WindowSeconds: 60, BanSeconds: 1},
			Sensitive: TierConfig{Requests: 1000, WindowSeconds: 300, BanSeconds: 3600},
		},
	}
	g, attacksDB := newTestGuardWithAttacks(t, cfg, nil)

	// Global limit is 1: call 1 allowed, call 2 rejected — trips the
	// ban and creates episode 1.
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		_ = g.Authorize(w, request("203.0.113.7", "/a"), nil, &security.Route{Path: "/a"})
	}

	time.Sleep(2100 * time.Millisecond)
	g.FlushAttacks() // closes episode 1 (quiet past its grace period), evicts it from memory

	// The ban has expired, but the underlying window counter (60s) is
	// still running and already past its limit, so this next call is
	// rejected again immediately — a fresh ban trip, and since episode
	// 1 was evicted, a new in-memory state (and DB row) is created.
	w := httptest.NewRecorder()
	_ = g.Authorize(w, request("203.0.113.7", "/a"), nil, &security.Route{Path: "/a"})

	if got := countAttacks(t, attacksDB, "203.0.113.7"); got != 2 {
		t.Fatalf("ip_attacks rows for the IP after a second episode = %d, want 2 (a new row, not a reopened one)", got)
	}
}

func TestHydrate_ClosesAttacksOrphanedByAPreviousProcess(t *testing.T) {
	cfg := testConfig()
	attacksDB := freshAttacksDB(t)

	if _, err := attacksDB.Exec(
		`INSERT INTO ip_attacks (id, ip, tier, started_at, last_seen_at, hit_count, ban_count) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"orphan-1", "9.9.9.9", "global", time.Now().Format(timeLayout), time.Now().Format(timeLayout), 5, 1,
	); err != nil {
		t.Fatalf("seed orphaned attack: %v", err)
	}

	_, err := NewWithDB(cfg, &spyInner{}, freshDB(t), nil, mustAttacksHandle(t, attacksDB), nil, nil)
	if err != nil {
		t.Fatalf("NewWithDB() error = %v", err)
	}

	var endedAt sql.NullString
	if err := attacksDB.QueryRow(`SELECT ended_at FROM ip_attacks WHERE id = ?`, "orphan-1").Scan(&endedAt); err != nil {
		t.Fatalf("query orphaned attack: %v", err)
	}
	if !endedAt.Valid {
		t.Fatal("an attack left open by a previous process instance must be closed at startup")
	}
}

func TestSweeper_FlushesAttacksOnTick(t *testing.T) {
	cfg := testConfig() // global limit 3: calls 1-3 allowed, call 4+ rejected
	g, attacksDB := newTestGuardWithAttacks(t, cfg, nil)

	for i := 0; i < 6; i++ {
		w := httptest.NewRecorder()
		_ = g.Authorize(w, request("203.0.113.7", "/a"), nil, &security.Route{Path: "/a"})
	}

	sweeper := NewSweeperWithInterval(g, nil, 10*time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	sweeper.Run(ctx)

	var hitCount int
	if err := attacksDB.QueryRow(`SELECT hit_count FROM ip_attacks WHERE ip = ?`, "203.0.113.7").Scan(&hitCount); err != nil {
		t.Fatalf("query attack row: %v", err)
	}
	if hitCount != 3 { // calls 4, 5, 6 are the rejections
		t.Fatalf("hit_count after sweeper tick = %d, want 3 (sweeper must flush attacks too)", hitCount)
	}
}

func TestRecordRecon_CreatesUnmatchedTierEpisode(t *testing.T) {
	cfg := testConfig()
	g, attacksDB := newTestGuardWithAttacks(t, cfg, nil)

	g.RecordRecon("203.0.113.7", request("203.0.113.7", "/wp-admin"))

	if got := countAttacks(t, attacksDB, "203.0.113.7"); got != 1 {
		t.Fatalf("ip_attacks rows for the IP = %d, want exactly 1", got)
	}
	var tier string
	if err := attacksDB.QueryRow(`SELECT tier FROM ip_attacks WHERE ip = ?`, "203.0.113.7").Scan(&tier); err != nil {
		t.Fatalf("query tier: %v", err)
	}
	if tier != "unmatched" {
		t.Fatalf("tier = %q, want %q", tier, "unmatched")
	}
}

func TestRecordRecon_MultipleHitsAggregateIntoOneEpisode(t *testing.T) {
	cfg := testConfig()
	g, attacksDB := newTestGuardWithAttacks(t, cfg, nil)

	for _, path := range []string{"/wp-admin", "/.env", "/wp-admin"} {
		g.RecordRecon("203.0.113.7", request("203.0.113.7", path))
	}
	g.FlushAttacks()

	if got := countAttacks(t, attacksDB, "203.0.113.7"); got != 1 {
		t.Fatalf("ip_attacks rows for the IP = %d, want exactly 1 (one episode, not one per probe)", got)
	}
	var hitCount int
	if err := attacksDB.QueryRow(`SELECT hit_count FROM ip_attacks WHERE ip = ?`, "203.0.113.7").Scan(&hitCount); err != nil {
		t.Fatalf("query hit_count: %v", err)
	}
	if hitCount != 3 {
		t.Fatalf("hit_count = %d, want 3", hitCount)
	}

	rows, err := attacksDB.Query(`SELECT path, hit_count FROM ip_attack_targets ORDER BY path`)
	if err != nil {
		t.Fatalf("query targets: %v", err)
	}
	defer rows.Close()
	counts := map[string]int{}
	for rows.Next() {
		var path string
		var hitCount int
		if err := rows.Scan(&path, &hitCount); err != nil {
			t.Fatalf("scan target: %v", err)
		}
		counts[path] = hitCount
	}
	if counts["/wp-admin"] != 2 {
		t.Fatalf("hit count for /wp-admin = %d, want 2", counts["/wp-admin"])
	}
	if counts["/.env"] != 1 {
		t.Fatalf("hit count for /.env = %d, want 1", counts["/.env"])
	}
}

// TestRecordRecon_NeverBansOrRateLimits confirms recon hits are purely
// observational — even far more hits than the global tier's own limit
// must never trip a ban or affect the rate limiter's own counters.
func TestRecordRecon_NeverBansOrRateLimits(t *testing.T) {
	cfg := testConfig() // global limit 3
	g := newTestGuard(t, cfg)

	for i := 0; i < 20; i++ {
		g.RecordRecon("203.0.113.7", request("203.0.113.7", "/wp-admin"))
	}

	if banned, _, _, _ := g.checkBanned("203.0.113.7"); banned {
		t.Fatal("RecordRecon must never ban the IP, however many hits")
	}

	// A normal request against a real route must still be allowed —
	// recon hits must not share the global limiter's counter.
	w := httptest.NewRecorder()
	if err := g.Authorize(w, request("203.0.113.7", "/anything"), nil, &security.Route{Path: "/anything"}); err != nil {
		t.Fatalf("Authorize() after only recon hits should succeed, error = %v", err)
	}
}

func TestRecordRecon_SkipsAllowlistedIP(t *testing.T) {
	cfg := testConfig()
	g, attacksDB := newTestGuardWithAttacks(t, cfg, nil)

	if err := g.AllowIP("9.9.9.9", "office egress", "admin-1"); err != nil {
		t.Fatalf("AllowIP() error = %v", err)
	}
	g.RecordRecon("9.9.9.9", request("9.9.9.9", "/wp-admin"))

	if got := countAttacks(t, attacksDB, "9.9.9.9"); got != 0 {
		t.Fatalf("ip_attacks rows for an allowlisted IP = %d, want 0", got)
	}
}
