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
	g, err := NewWithDB(cfg, &spyInner{}, freshDB(t), nil, freshAttacksDB(t), nil, fw)
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

func TestFirewallStatus_NilFirewallReportsUnavailable(t *testing.T) {
	g := newTestGuard(t, testConfig())
	name, available, detail := g.FirewallStatus()
	if name != "none" || available || detail == "" {
		t.Fatalf("FirewallStatus() = (%q, %v, %q), want (none, false, non-empty)", name, available, detail)
	}
}
