//go:build windows

package firewall

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"time"

	"github.com/a-digi/coco-logger/logger"
)

const commandTimeout = 2 * time.Second

type commandRunner func(ctx context.Context, name string, args ...string) ([]byte, error)

func realRunner(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

// windowsBanner — dev-machine parity only; production is confirmed
// Linux (see plan section 14's "production reality check"). Uses
// `netsh` (ships on every Windows install) rather than PowerShell —
// cheaper to spawn, no extra dependency. Requires an elevated
// (Administrator) process; Available() probes that with a harmless
// read-only command rather than assuming it.
type windowsBanner struct {
	log       logger.Logger
	run       commandRunner
	available bool
	detail    string
}

func detectPlatform(log logger.Logger) Banner {
	return newWindowsBanner(log, realRunner)
}

func newWindowsBanner(log logger.Logger, run commandRunner) *windowsBanner {
	b := &windowsBanner{log: log, run: run}
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	if _, err := run(ctx, "netsh", "advfirewall", "show", "currentprofile"); err != nil {
		b.detail = "netsh advfirewall not usable — this process may need to run elevated (Administrator)"
		return b
	}
	b.available = true
	return b
}

func (b *windowsBanner) Name() string {
	if !b.available {
		return "none"
	}
	return "windows_firewall"
}

func (b *windowsBanner) Available() bool { return b.available }
func (b *windowsBanner) Detail() string  { return b.detail }

// ruleName is deterministic per IP, so Unban can delete-by-name
// without having to track a separate id.
func ruleName(ip string) string {
	return "coco-iam-ban-" + ip
}

func (b *windowsBanner) Ban(ip string, duration time.Duration) error {
	return b.runNetsh(ip, "advfirewall", "firewall", "add", "rule",
		"name="+ruleName(ip), "dir=in", "action=block", "remoteip="+ip)
}

func (b *windowsBanner) Unban(ip string) error {
	return b.runNetsh(ip, "advfirewall", "firewall", "delete", "rule", "name="+ruleName(ip))
}

// ListBannedIPs lists rules and extracts the IP from names this
// backend's own Ban() created (ruleName's "coco-iam-ban-<ip>"
// convention) — exact, unlike the Linux heuristic, since the rule name
// is entirely under this backend's control.
func (b *windowsBanner) ListBannedIPs() ([]string, error) {
	if !b.available {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	out, err := b.run(ctx, "netsh", "advfirewall", "firewall", "show", "rule", "name=all")
	if err != nil {
		return nil, fmt.Errorf("firewall: netsh advfirewall firewall show rule name=all: %w (output: %s)", err, string(out))
	}
	prefix := ruleName("")
	var ips []string
	for _, line := range strings.Split(string(out), "\n") {
		idx := strings.Index(line, "Rule Name:")
		if idx == -1 {
			continue
		}
		value := strings.TrimSpace(line[idx+len("Rule Name:"):])
		if !strings.HasPrefix(value, prefix) {
			continue
		}
		ip := strings.TrimPrefix(value, prefix)
		if net.ParseIP(ip) == nil {
			continue
		}
		ips = append(ips, ip)
	}
	return ips, nil
}

// RemoveAllRules deletes every rule named ruleName(ip) — netsh's
// delete-by-name removes every matching rule in one call, not just
// one, so this collapses any duplicate `add rule` calls in a single
// invocation. Counts existing occurrences first so the caller gets an
// accurate removed count.
func (b *windowsBanner) RemoveAllRules(ip string) (int, error) {
	if !b.available {
		return 0, nil
	}
	if err := validateIP(ip); err != nil {
		return 0, err
	}
	existing, err := b.ListBannedIPs()
	if err != nil {
		return 0, err
	}
	count := 0
	for _, e := range existing {
		if e == ip {
			count++
		}
	}
	if count == 0 {
		return 0, nil
	}
	if err := b.Unban(ip); err != nil {
		return 0, err
	}
	return count, nil
}

func (b *windowsBanner) runNetsh(ip string, args ...string) error {
	if !b.available {
		return fmt.Errorf("firewall: windows firewall not available (%s)", b.detail)
	}
	if err := validateIP(ip); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	out, err := b.run(ctx, "netsh", args...)
	if err != nil {
		return fmt.Errorf("firewall: netsh %v: %w (output: %s)", args, err, string(out))
	}
	return nil
}
