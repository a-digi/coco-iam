//go:build linux

package firewall

import (
	"context"
	"errors"
	"testing"
	"time"
)

// recordingRunner captures every command invocation instead of really
// executing anything.
type recordingRunner struct {
	calls  [][]string
	err    error
	output []byte
}

func (r *recordingRunner) run(ctx context.Context, name string, args ...string) ([]byte, error) {
	call := append([]string{name}, args...)
	r.calls = append(r.calls, call)
	return r.output, r.err
}

func foundLookPath(file string) (string, error) { return "/sbin/" + file, nil }
func missingLookPath(file string) (string, error) {
	return "", errors.New("executable file not found in $PATH")
}

func TestLinuxBanner_AvailableWhenIPTablesFound(t *testing.T) {
	r := &recordingRunner{}
	b := newLinuxBanner(nil, r.run, foundLookPath)
	if !b.Available() {
		t.Fatal("expected Available() = true when iptables is found")
	}
	if b.Name() != "iptables" {
		t.Fatalf("Name() = %q, want %q", b.Name(), "iptables")
	}
	if b.Detail() != "" {
		t.Fatalf("Detail() = %q, want empty when available", b.Detail())
	}
}

func TestLinuxBanner_UnavailableWhenIPTablesMissing(t *testing.T) {
	r := &recordingRunner{}
	b := newLinuxBanner(nil, r.run, missingLookPath)
	if b.Available() {
		t.Fatal("expected Available() = false when iptables is missing")
	}
	if b.Name() != "none" {
		t.Fatalf("Name() = %q, want %q", b.Name(), "none")
	}
	if b.Detail() == "" {
		t.Fatal("expected a non-empty Detail() explaining why iptables is unavailable")
	}
}

func TestLinuxBanner_Ban_BuildsCorrectCommand(t *testing.T) {
	r := &recordingRunner{}
	b := newLinuxBanner(nil, r.run, foundLookPath)

	if err := b.Ban("203.0.113.7", time.Minute); err != nil {
		t.Fatalf("Ban() error = %v", err)
	}
	if len(r.calls) != 1 {
		t.Fatalf("expected exactly 1 command invocation, got %d", len(r.calls))
	}
	want := []string{"iptables", "-I", "INPUT", "-s", "203.0.113.7", "-j", "DROP"}
	if !equalSlices(r.calls[0], want) {
		t.Fatalf("command = %v, want %v", r.calls[0], want)
	}
}

func TestLinuxBanner_Unban_BuildsCorrectCommand(t *testing.T) {
	r := &recordingRunner{}
	b := newLinuxBanner(nil, r.run, foundLookPath)

	if err := b.Unban("203.0.113.7"); err != nil {
		t.Fatalf("Unban() error = %v", err)
	}
	want := []string{"iptables", "-D", "INPUT", "-s", "203.0.113.7", "-j", "DROP"}
	if !equalSlices(r.calls[0], want) {
		t.Fatalf("command = %v, want %v", r.calls[0], want)
	}
}

func TestLinuxBanner_RejectsInvalidIPBeforeRunningAnyCommand(t *testing.T) {
	r := &recordingRunner{}
	b := newLinuxBanner(nil, r.run, foundLookPath)

	maliciousInputs := []string{
		"203.0.113.7; rm -rf /",
		"203.0.113.7 -j ACCEPT",
		"$(reboot)",
		"not-an-ip",
	}
	for _, ip := range maliciousInputs {
		if err := b.Ban(ip, time.Minute); err == nil {
			t.Errorf("Ban(%q) = nil error, want rejection", ip)
		}
		if err := b.Unban(ip); err == nil {
			t.Errorf("Unban(%q) = nil error, want rejection", ip)
		}
	}
	if len(r.calls) != 0 {
		t.Fatalf("expected zero commands run for invalid input, got %d: %v", len(r.calls), r.calls)
	}
}

func TestLinuxBanner_UnavailableBackendErrorsWithoutRunningACommand(t *testing.T) {
	r := &recordingRunner{}
	b := newLinuxBanner(nil, r.run, missingLookPath)

	if err := b.Ban("203.0.113.7", time.Minute); err == nil {
		t.Fatal("expected an error when iptables is unavailable")
	}
	if len(r.calls) != 0 {
		t.Fatalf("expected zero commands run when unavailable, got %d", len(r.calls))
	}
}

func TestLinuxBanner_PropagatesCommandFailure(t *testing.T) {
	r := &recordingRunner{err: errors.New("exit status 4")}
	b := newLinuxBanner(nil, r.run, foundLookPath)

	if err := b.Ban("203.0.113.7", time.Minute); err == nil {
		t.Fatal("expected the underlying command error to propagate")
	}
}

func TestLinuxBanner_ListBannedIPs_ParsesDropRulesWithSingleIPSource(t *testing.T) {
	r := &recordingRunner{output: []byte(
		"Chain INPUT (policy ACCEPT)\n" +
			"target     prot opt source               destination\n" +
			"DROP       all  --  203.0.113.7          0.0.0.0/0\n" +
			"ACCEPT     all  --  198.51.100.0/24       0.0.0.0/0\n" + // not DROP -> ignored
			"DROP       all  --  198.51.100.0/24       0.0.0.0/0\n" + // CIDR, not a single IP -> ignored
			"DROP       all  --  203.0.113.9          0.0.0.0/0\n",
	)}
	b := newLinuxBanner(nil, r.run, foundLookPath)

	ips, err := b.ListBannedIPs()
	if err != nil {
		t.Fatalf("ListBannedIPs() error = %v", err)
	}
	want := []string{"203.0.113.7", "203.0.113.9"}
	if !equalSlices(ips, want) {
		t.Fatalf("ListBannedIPs() = %v, want %v", ips, want)
	}
}

func TestLinuxBanner_ListBannedIPs_EmptyWhenUnavailable(t *testing.T) {
	r := &recordingRunner{}
	b := newLinuxBanner(nil, r.run, missingLookPath)

	ips, err := b.ListBannedIPs()
	if err != nil {
		t.Fatalf("ListBannedIPs() error = %v", err)
	}
	if len(ips) != 0 {
		t.Fatalf("ListBannedIPs() = %v, want empty when unavailable", ips)
	}
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
