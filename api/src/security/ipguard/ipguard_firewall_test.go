package ipguard

import (
	"errors"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/a-digi/coco-server/server/security"
)

var errFirewallDown = errors.New("fake firewall: simulated failure")

// fakeFirewall is a firewall.Banner test double recording every
// Ban/Unban call instead of touching a real OS firewall.
type fakeFirewall struct {
	mu       sync.Mutex
	banned   map[string]bool
	banCalls int
	banErr   error
	unbanErr error
}

func newFakeFirewall() *fakeFirewall {
	return &fakeFirewall{banned: make(map[string]bool)}
}

func (f *fakeFirewall) Ban(ip string, duration time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.banCalls++
	if f.banErr != nil {
		return f.banErr
	}
	f.banned[ip] = true
	return nil
}

func (f *fakeFirewall) Unban(ip string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.unbanErr != nil {
		return f.unbanErr
	}
	delete(f.banned, ip)
	return nil
}

func (f *fakeFirewall) Name() string    { return "fake" }
func (f *fakeFirewall) Available() bool { return true }
func (f *fakeFirewall) Detail() string  { return "" }
func (f *fakeFirewall) ListBannedIPs() ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	ips := make([]string, 0, len(f.banned))
	for ip := range f.banned {
		ips = append(ips, ip)
	}
	return ips, nil
}
func (f *fakeFirewall) RemoveAllRules(ip string) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.banned[ip] {
		return 0, nil
	}
	delete(f.banned, ip)
	return 1, nil
}
func (f *fakeFirewall) banCallCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.banCalls
}
func (f *fakeFirewall) isBanned(ip string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.banned[ip]
}

// waitFor polls cond briefly — Ban/Unban fire the firewall call in a
// background goroutine (see plan section 14's "Execution safety"
// note), so tests need to wait for it rather than checking
// synchronously.
func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not met within timeout")
}

func newTestGuardWithFirewall(t *testing.T, cfg Config, fw *fakeFirewall) *IPGuardSecurityLayer {
	t.Helper()
	g, err := NewWithDB(cfg, &spyInner{}, freshDB(t), nil, freshAttacksHandle(t), nil, fw, nil)
	if err != nil {
		t.Fatalf("NewWithDB() error = %v", err)
	}
	return g
}

func TestAutoBan_TriggersFirewallBan(t *testing.T) {
	fw := newFakeFirewall()
	g := newTestGuardWithFirewall(t, testConfig(), fw) // global limit 3

	route := &security.Route{Path: "/a"}
	for i := 0; i < 4; i++ { // 4th call trips the global ban
		w := httptest.NewRecorder()
		_ = g.Authorize(w, request("203.0.113.7", "/a"), nil, route)
	}

	waitFor(t, func() bool { return fw.isBanned("203.0.113.7") })
}

func TestManualBan_AlsoTriggersFirewallBan(t *testing.T) {
	fw := newFakeFirewall()
	g := newTestGuardWithFirewall(t, testConfig(), fw)

	if err := g.Ban("198.51.100.5", "manual", "manually banned", time.Hour, nil); err != nil {
		t.Fatalf("Ban() error = %v", err)
	}

	waitFor(t, func() bool { return fw.isBanned("198.51.100.5") })
}

// TestBanFirewall_SkipsWhenAlreadyBannedAtOSLevel is the actual fix
// for the reported duplicate-rules symptom: re-banning an IP that
// already has an OS-level rule (a repeated attack retrigger, clicking
// "Resync now" more than once, or the automatic startup resync running
// on every restart) must not insert a second rule.
func TestBanFirewall_SkipsWhenAlreadyBannedAtOSLevel(t *testing.T) {
	fw := newFakeFirewall()
	g := newTestGuardWithFirewall(t, testConfig(), fw)

	if err := g.Ban("198.51.100.5", "manual", "test", time.Hour, nil); err != nil {
		t.Fatalf("first Ban() error = %v", err)
	}
	waitFor(t, func() bool { return fw.isBanned("198.51.100.5") })
	callsAfterFirst := fw.banCallCount()

	if err := g.Ban("198.51.100.5", "manual", "test", time.Hour, nil); err != nil {
		t.Fatalf("second Ban() error = %v", err)
	}
	// banFirewall's check-then-ban runs in a goroutine — give it a
	// moment, then assert no further call happened (can't waitFor a
	// negative, so a short fixed sleep is the pragmatic choice here).
	time.Sleep(50 * time.Millisecond)

	if got := fw.banCallCount(); got != callsAfterFirst {
		t.Fatalf("expected no additional firewall.Ban() call for an already-banned IP, calls went from %d to %d", callsAfterFirst, got)
	}
}

func TestUnban_LiftsFirewallBan(t *testing.T) {
	fw := newFakeFirewall()
	g := newTestGuardWithFirewall(t, testConfig(), fw)

	if err := g.Ban("198.51.100.5", "manual", "test", time.Hour, nil); err != nil {
		t.Fatalf("Ban() error = %v", err)
	}
	waitFor(t, func() bool { return fw.isBanned("198.51.100.5") })

	if err := g.Unban("198.51.100.5"); err != nil {
		t.Fatalf("Unban() error = %v", err)
	}
	waitFor(t, func() bool { return !fw.isBanned("198.51.100.5") })
}

func TestPruneExpiredBans_LiftsFirewallBanOnExpiry(t *testing.T) {
	fw := newFakeFirewall()
	g := newTestGuardWithFirewall(t, testConfig(), fw)

	if err := g.Ban("198.51.100.5", "global", "test", -time.Minute, nil); err != nil { // already expired
		t.Fatalf("Ban() error = %v", err)
	}
	waitFor(t, func() bool { return fw.isBanned("198.51.100.5") })

	if err := g.PruneExpiredBans(); err != nil {
		t.Fatalf("PruneExpiredBans() error = %v", err)
	}
	waitFor(t, func() bool { return !fw.isBanned("198.51.100.5") })
}

func TestFirewallFailure_DoesNotBreakEnforcement(t *testing.T) {
	fw := newFakeFirewall()
	fw.banErr = errFirewallDown
	g := newTestGuardWithFirewall(t, testConfig(), fw)

	// Even though the firewall call will fail, the in-memory/DB ban
	// must still take effect — application-layer enforcement must
	// never depend on the OS-level call succeeding.
	route := &security.Route{Path: "/a"}
	for i := 0; i < 4; i++ {
		w := httptest.NewRecorder()
		_ = g.Authorize(w, request("203.0.113.7", "/a"), nil, route)
	}

	w := httptest.NewRecorder()
	if err := g.Authorize(w, request("203.0.113.7", "/a"), nil, route); err == nil {
		t.Fatal("expected the IP to still be blocked at the application layer despite the firewall call failing")
	}
}

func TestNilFirewall_IsANoOp(t *testing.T) {
	g := newTestGuard(t, testConfig()) // newTestGuard passes nil for fw
	if err := g.Ban("203.0.113.7", "manual", "test", time.Hour, nil); err != nil {
		t.Fatalf("Ban() with nil firewall error = %v", err)
	}
	if err := g.Unban("203.0.113.7"); err != nil {
		t.Fatalf("Unban() with nil firewall error = %v", err)
	}
}

func TestFirewallStatus_ReportsBackendState(t *testing.T) {
	fw := newFakeFirewall()
	g := newTestGuardWithFirewall(t, testConfig(), fw)

	name, available, detail := g.FirewallStatus()
	if name != "fake" || !available || detail != "" {
		t.Fatalf("FirewallStatus() = (%q, %v, %q), want (fake, true, \"\")", name, available, detail)
	}
}

// TestRemoveAllFirewallRules_AlsoUnbansWhenStillActivelyBanned is the
// current cascade-unban behavior: removing an OS-level rule for an IP
// the DB still actively bans also deletes that ban row — "remove from
// the Firewall page" means fully unban, not "re-apply a clean rule" (a
// prior version of this method did the latter, which from an admin's
// perspective looked exactly like Remove silently not working, since
// the rule reappeared immediately).
func TestRemoveAllFirewallRules_AlsoUnbansWhenStillActivelyBanned(t *testing.T) {
	fw := newFakeFirewall()
	g := newTestGuardWithFirewall(t, testConfig(), fw)

	if err := g.Ban("198.51.100.5", "manual", "test", time.Hour, nil); err != nil {
		t.Fatalf("Ban() error = %v", err)
	}
	waitFor(t, func() bool { return fw.isBanned("198.51.100.5") })

	removed, alsoUnbanned, err := g.RemoveAllFirewallRules("198.51.100.5")
	if err != nil {
		t.Fatalf("RemoveAllFirewallRules() error = %v", err)
	}
	if removed != 1 {
		t.Fatalf("removed = %d, want 1", removed)
	}
	if !alsoUnbanned {
		t.Fatal("expected alsoUnbanned = true since the IP was still actively banned in the DB")
	}
	if fw.isBanned("198.51.100.5") {
		t.Fatal("expected the firewall to stay clear — no rule should be re-applied")
	}
	bans, err := g.banQuery.ListActive(time.Now())
	if err != nil {
		t.Fatalf("ListActive() error = %v", err)
	}
	for _, b := range bans {
		if b.IP == "198.51.100.5" {
			t.Fatal("expected the DB ban row to be deleted too")
		}
	}
}

// TestRemoveAllFirewallRules_DoesNotUnbanWhenNotActivelyBanned covers
// the other half: an IP with a leftover OS-level rule but no active DB
// ban (e.g. the ban already expired and was pruned, or was deleted by
// some other path) has nothing left to cascade-delete.
func TestRemoveAllFirewallRules_DoesNotUnbanWhenNotActivelyBanned(t *testing.T) {
	fw := newFakeFirewall()
	g := newTestGuardWithFirewall(t, testConfig(), fw)

	if err := g.Ban("198.51.100.5", "manual", "test", time.Hour, nil); err != nil {
		t.Fatalf("Ban() error = %v", err)
	}
	waitFor(t, func() bool { return fw.isBanned("198.51.100.5") })

	// Simulate a stale OS-level rule: the DB row is gone (e.g. deleted
	// through some other path) but the firewall rule wasn't cleaned up.
	if err := g.banPersist.DeleteBan("198.51.100.5"); err != nil {
		t.Fatalf("DeleteBan() error = %v", err)
	}

	removed, alsoUnbanned, err := g.RemoveAllFirewallRules("198.51.100.5")
	if err != nil {
		t.Fatalf("RemoveAllFirewallRules() error = %v", err)
	}
	if removed != 1 {
		t.Fatalf("removed = %d, want 1", removed)
	}
	if alsoUnbanned {
		t.Fatal("expected alsoUnbanned = false since the IP has no active DB ban")
	}
	if fw.isBanned("198.51.100.5") {
		t.Fatal("expected the firewall rule to stay removed")
	}
}

func TestFirewallStatus_NilFirewallReportsUnavailable(t *testing.T) {
	g := newTestGuard(t, testConfig())
	name, available, detail := g.FirewallStatus()
	if name != "none" || available || detail == "" {
		t.Fatalf("FirewallStatus() = (%q, %v, %q), want (none, false, non-empty)", name, available, detail)
	}
}

// TestHydrate_ResyncsActiveBansIntoFreshFirewall reproduces a redeploy:
// the DB survives (bans stay active), but the OS firewall doesn't — a
// fresh process starts with no rules of its own. hydrate() must
// re-apply every still-active ban into whatever firewall backend the
// new process has, without needing anyone to click "Resync now".
func TestHydrate_ResyncsActiveBansIntoFreshFirewall(t *testing.T) {
	db := freshDB(t) // shared across both "processes" below, like a real DB file surviving a restart

	fw1 := newFakeFirewall()
	g1, err := NewWithDB(testConfig(), &spyInner{}, db, nil, freshAttacksHandle(t), nil, fw1, nil)
	if err != nil {
		t.Fatalf("NewWithDB() (process 1) error = %v", err)
	}
	if err := g1.Ban("198.51.100.9", "manual", "test", time.Hour, nil); err != nil {
		t.Fatalf("Ban() error = %v", err)
	}
	waitFor(t, func() bool { return fw1.isBanned("198.51.100.9") })

	// Simulate a restart: same DB, brand-new IPGuardSecurityLayer, and a
	// brand-new firewall backend that has never heard of this IP.
	fw2 := newFakeFirewall()
	g2, err := NewWithDB(testConfig(), &spyInner{}, db, nil, freshAttacksHandle(t), nil, fw2, nil)
	if err != nil {
		t.Fatalf("NewWithDB() (process 2) error = %v", err)
	}
	_ = g2

	waitFor(t, func() bool { return fw2.isBanned("198.51.100.9") })
}

// TestHydrate_DoesNotResyncExpiredBans ensures the startup resync
// respects expiry — an already-expired row (which the sweeper hasn't
// pruned yet) must not be re-applied to a fresh firewall backend.
func TestHydrate_DoesNotResyncExpiredBans(t *testing.T) {
	db := freshDB(t)

	fw1 := newFakeFirewall()
	g1, err := NewWithDB(testConfig(), &spyInner{}, db, nil, freshAttacksHandle(t), nil, fw1, nil)
	if err != nil {
		t.Fatalf("NewWithDB() (process 1) error = %v", err)
	}
	if err := g1.Ban("198.51.100.9", "global", "test", -time.Minute, nil); err != nil { // already expired
		t.Fatalf("Ban() error = %v", err)
	}
	waitFor(t, func() bool { return fw1.isBanned("198.51.100.9") })

	fw2 := newFakeFirewall()
	if _, err := NewWithDB(testConfig(), &spyInner{}, db, nil, freshAttacksHandle(t), nil, fw2, nil); err != nil {
		t.Fatalf("NewWithDB() (process 2) error = %v", err)
	}

	time.Sleep(50 * time.Millisecond) // let any (wrongly-fired) goroutine settle
	if fw2.isBanned("198.51.100.9") {
		t.Fatal("expired ban must not be resynced into a fresh firewall backend")
	}
}
