//go:build windows

package firewall

import (
	"context"
	"fmt"
	"os/exec"
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
