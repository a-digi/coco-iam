//go:build darwin

package firewall

import (
	"context"
	"errors"
	"testing"
	"time"
)

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

func TestDarwinBanner_AvailableWhenPfctlFoundAndTableLoaded(t *testing.T) {
	r := &recordingRunner{}
	b := newDarwinBanner(nil, r.run, foundLookPath)
	if !b.Available() {
		t.Fatalf("expected Available() = true, detail = %q", b.Detail())
	}
	if b.Name() != "pf" {
		t.Fatalf("Name() = %q, want %q", b.Name(), "pf")
	}
	if b.Detail() != "" {
		t.Fatalf("Detail() = %q, want empty when available", b.Detail())
	}
	// newDarwinBanner's detection itself runs one "-T show" probe.
	if len(r.calls) != 1 {
		t.Fatalf("expected exactly 1 detection probe, got %d: %v", len(r.calls), r.calls)
	}
}

func TestDarwinBanner_UnavailableWhenPfctlMissing(t *testing.T) {
	r := &recordingRunner{}
	b := newDarwinBanner(nil, r.run, missingLookPath)
	if b.Available() {
		t.Fatal("expected Available() = false when pfctl is missing")
	}
	if b.Name() != "none" {
		t.Fatalf("Name() = %q, want %q", b.Name(), "none")
	}
	if b.Detail() == "" {
		t.Fatal("expected a non-empty Detail()")
	}
	if len(r.calls) != 0 {
		t.Fatalf("expected no probe command run when pfctl itself is missing, got %d", len(r.calls))
	}
}

func TestDarwinBanner_UnavailableWhenTableNotLoaded(t *testing.T) {
	r := &recordingRunner{err: errors.New("pfctl: table does not exist")}
	b := newDarwinBanner(nil, r.run, foundLookPath)
	if b.Available() {
		t.Fatal("expected Available() = false when the pf table isn't loaded")
	}
	if b.Detail() == "" {
		t.Fatal("expected a non-empty Detail() explaining the missing one-time setup")
	}
}

func TestDarwinBanner_Ban_BuildsCorrectCommand(t *testing.T) {
	r := &recordingRunner{}
	b := newDarwinBanner(nil, r.run, foundLookPath)
	r.calls = nil // clear the detection probe call

	if err := b.Ban("203.0.113.7", time.Minute); err != nil {
		t.Fatalf("Ban() error = %v", err)
	}
	want := []string{"pfctl", "-t", pfTable, "-T", "add", "203.0.113.7"}
	if !equalSlices(r.calls[0], want) {
		t.Fatalf("command = %v, want %v", r.calls[0], want)
	}
}

func TestDarwinBanner_Unban_BuildsCorrectCommand(t *testing.T) {
	r := &recordingRunner{}
	b := newDarwinBanner(nil, r.run, foundLookPath)
	r.calls = nil

	if err := b.Unban("203.0.113.7"); err != nil {
		t.Fatalf("Unban() error = %v", err)
	}
	want := []string{"pfctl", "-t", pfTable, "-T", "delete", "203.0.113.7"}
	if !equalSlices(r.calls[0], want) {
		t.Fatalf("command = %v, want %v", r.calls[0], want)
	}
}

func TestDarwinBanner_RejectsInvalidIPBeforeRunningAnyCommand(t *testing.T) {
	r := &recordingRunner{}
	b := newDarwinBanner(nil, r.run, foundLookPath)
	r.calls = nil

	for _, ip := range []string{"203.0.113.7; rm -rf /", "not-an-ip", "$(reboot)"} {
		if err := b.Ban(ip, time.Minute); err == nil {
			t.Errorf("Ban(%q) = nil error, want rejection", ip)
		}
		if err := b.Unban(ip); err == nil {
			t.Errorf("Unban(%q) = nil error, want rejection", ip)
		}
	}
	if len(r.calls) != 0 {
		t.Fatalf("expected zero commands for invalid input, got %d: %v", len(r.calls), r.calls)
	}
}

func TestDarwinBanner_UnavailableBackendErrorsWithoutRunningACommand(t *testing.T) {
	r := &recordingRunner{}
	b := newDarwinBanner(nil, r.run, missingLookPath)

	if err := b.Ban("203.0.113.7", time.Minute); err == nil {
		t.Fatal("expected an error when pf is unavailable")
	}
	if len(r.calls) != 0 {
		t.Fatalf("expected zero commands run when unavailable, got %d", len(r.calls))
	}
}

func TestDarwinBanner_ListBannedIPs_ParsesTableEntries(t *testing.T) {
	r := &recordingRunner{output: []byte("203.0.113.7\n198.51.100.5\n")}
	b := newDarwinBanner(nil, r.run, foundLookPath)
	r.calls = nil

	ips, err := b.ListBannedIPs()
	if err != nil {
		t.Fatalf("ListBannedIPs() error = %v", err)
	}
	want := []string{"203.0.113.7", "198.51.100.5"}
	if !equalSlices(ips, want) {
		t.Fatalf("ListBannedIPs() = %v, want %v", ips, want)
	}
}

func TestDarwinBanner_ListBannedIPs_EmptyWhenUnavailable(t *testing.T) {
	r := &recordingRunner{}
	b := newDarwinBanner(nil, r.run, missingLookPath)

	ips, err := b.ListBannedIPs()
	if err != nil {
		t.Fatalf("ListBannedIPs() error = %v", err)
	}
	if len(ips) != 0 {
		t.Fatalf("ListBannedIPs() = %v, want empty when unavailable", ips)
	}
}

func TestDarwinBanner_RemoveAllRules_RemovesWhenPresent(t *testing.T) {
	r := &recordingRunner{output: []byte("203.0.113.7\n198.51.100.5\n")}
	b := newDarwinBanner(nil, r.run, foundLookPath)
	r.calls = nil

	removed, err := b.RemoveAllRules("203.0.113.7")
	if err != nil {
		t.Fatalf("RemoveAllRules() error = %v", err)
	}
	if removed != 1 {
		t.Fatalf("removed = %d, want 1", removed)
	}
}

func TestDarwinBanner_RemoveAllRules_ZeroWhenNotPresent(t *testing.T) {
	r := &recordingRunner{output: []byte("198.51.100.5\n")}
	b := newDarwinBanner(nil, r.run, foundLookPath)
	r.calls = nil

	removed, err := b.RemoveAllRules("203.0.113.7")
	if err != nil {
		t.Fatalf("RemoveAllRules() error = %v", err)
	}
	if removed != 0 {
		t.Fatalf("removed = %d, want 0", removed)
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
