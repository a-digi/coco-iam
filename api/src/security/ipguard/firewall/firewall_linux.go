//go:build linux

package firewall

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/a-digi/coco-logger/logger"
)

// commandTimeout bounds every OS command this backend runs — Ban/
// Unban must never hang the caller indefinitely (e.g. iptables
// blocking on a contended xtables lock). See plan section 14's
// "Execution safety" note.
const commandTimeout = 2 * time.Second

// commandRunner is the seam that lets tests substitute a fake process
// runner instead of really invoking exec.Command — the same pattern
// as Limiter's injectable Clock.
type commandRunner func(ctx context.Context, name string, args ...string) ([]byte, error)

func realRunner(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

// linuxBanner covers Debian, Ubuntu, and Alpine identically — all
// three ship the Linux kernel's netfilter and the same `iptables`
// command surface once the package is installed. nftables-only hosts
// (no `iptables` compat binary) are not supported by this backend —
// see plan section 14: nft's delete-by-handle model (you must look up
// a rule's numeric handle before removing it, unlike iptables' exact
// symmetric -I/-D) needs real design work this phase doesn't attempt;
// shipping a half-correct nft path was judged worse than not having
// one. Available() reports false with a clear reason if iptables
// isn't found — enforcement still works at the application layer.
type linuxBanner struct {
	log       logger.Logger
	run       commandRunner
	available bool
	detail    string
}

// lookPathFunc mirrors exec.LookPath's signature — injectable so
// tests can simulate "iptables not found" without needing an
// environment that actually lacks it.
type lookPathFunc func(file string) (string, error)

func detectPlatform(log logger.Logger) Banner {
	return newLinuxBanner(log, realRunner, exec.LookPath)
}

func newLinuxBanner(log logger.Logger, run commandRunner, lookPath lookPathFunc) *linuxBanner {
	b := &linuxBanner{log: log, run: run}
	if _, err := lookPath("iptables"); err == nil {
		b.available = true
	} else {
		b.detail = "iptables not found in PATH (on Alpine: apk add iptables)"
	}
	return b
}

func (b *linuxBanner) Name() string {
	if !b.available {
		return "none"
	}
	return "iptables"
}

func (b *linuxBanner) Available() bool { return b.available }
func (b *linuxBanner) Detail() string  { return b.detail }

// Ban inserts a DROP rule at the top of INPUT for ip — inserted (-I),
// not appended, so it takes priority over rules further down the
// chain.
func (b *linuxBanner) Ban(ip string, duration time.Duration) error {
	return b.runIPTables(ip, "-I", "INPUT", "-s", ip, "-j", "DROP")
}

// Unban removes the exact rule Ban added. iptables' -D matches by
// rule specification, so this is safe to call even if the rule was
// already removed some other way (e.g. an operator ran `iptables -F`)
// — it just errors, which the caller logs and moves on from; see
// plan section 15's "Unban must be idempotent and tolerant of drift".
func (b *linuxBanner) Unban(ip string) error {
	return b.runIPTables(ip, "-D", "INPUT", "-s", ip, "-j", "DROP")
}

// runIPTables validates ip explicitly (never trusting its position
// inside args) before ever building the command.
func (b *linuxBanner) runIPTables(ip string, args ...string) error {
	if !b.available {
		return fmt.Errorf("firewall: iptables not available (%s)", b.detail)
	}
	if err := validateIP(ip); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	out, err := b.run(ctx, "iptables", args...)
	if err != nil {
		return fmt.Errorf("firewall: iptables %v: %w (output: %s)", args, err, string(out))
	}
	return nil
}
