//go:build darwin

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

// pfTable is the persistent pf table this backend adds/removes IPs
// from. It must already exist — see this file's doc comment.
const pfTable = "coco_iam_banned"

type commandRunner func(ctx context.Context, name string, args ...string) ([]byte, error)

func realRunner(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

type lookPathFunc func(file string) (string, error)

// darwinBanner — dev-machine parity only; production is confirmed
// Linux (see plan section 14's "production reality check"). Unlike
// iptables, pf has no ad hoc "block this one IP" command: it works
// off a persistent table referenced by a rule already loaded into
// pf.conf. That requires a one-time manual host setup this code
// cannot safely automate (rewriting /etc/pf.conf programmatically
// risks clobbering the host's existing firewall rules):
//
//	# /etc/pf.conf — added once, by hand
//	table <coco_iam_banned> persist
//	block drop from <coco_iam_banned> to any
//
// then `pfctl -e` to enable pf if not already active. Available()
// checks both that pfctl exists AND that the table is actually
// loaded, so a skipped setup step is reported clearly rather than
// failing obscurely on the first real ban.
type darwinBanner struct {
	log       logger.Logger
	run       commandRunner
	available bool
	detail    string
}

func detectPlatform(log logger.Logger) Banner {
	return newDarwinBanner(log, realRunner, exec.LookPath)
}

func newDarwinBanner(log logger.Logger, run commandRunner, lookPath lookPathFunc) *darwinBanner {
	b := &darwinBanner{log: log, run: run}
	if _, err := lookPath("pfctl"); err != nil {
		b.detail = "pfctl not found in PATH"
		return b
	}
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	if _, err := run(ctx, "pfctl", "-t", pfTable, "-T", "show"); err != nil {
		b.detail = fmt.Sprintf(
			"pf table %q not loaded — see plan/ip-abuse-protection/plan.md section 14 for the one-time /etc/pf.conf setup",
			pfTable,
		)
		return b
	}
	b.available = true
	return b
}

func (b *darwinBanner) Name() string {
	if !b.available {
		return "none"
	}
	return "pf"
}

func (b *darwinBanner) Available() bool { return b.available }
func (b *darwinBanner) Detail() string  { return b.detail }

// Ban adds ip to the pf table — pf then drops matching traffic per
// the block rule already loaded in pf.conf.
func (b *darwinBanner) Ban(ip string, duration time.Duration) error {
	return b.runPfctl(ip, "-t", pfTable, "-T", "add", ip)
}

// Unban removes ip from the pf table.
func (b *darwinBanner) Unban(ip string) error {
	return b.runPfctl(ip, "-t", pfTable, "-T", "delete", ip)
}

// ListBannedIPs reads the pf table directly — unlike the Linux
// heuristic, this is exact: coco_iam_banned is a dedicated table this
// backend owns, not a shared chain another rule might also populate.
func (b *darwinBanner) ListBannedIPs() ([]string, error) {
	if !b.available {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	out, err := b.run(ctx, "pfctl", "-t", pfTable, "-T", "show")
	if err != nil {
		return nil, fmt.Errorf("firewall: pfctl -t %s -T show: %w (output: %s)", pfTable, err, string(out))
	}
	var ips []string
	for _, line := range strings.Split(string(out), "\n") {
		ip := strings.TrimSpace(line)
		if ip == "" {
			continue
		}
		if net.ParseIP(ip) == nil {
			continue
		}
		ips = append(ips, ip)
	}
	return ips, nil
}

// RemoveAllRules removes ip from the pf table. Unlike the Linux
// backend, pf tables are sets — an IP can appear at most once, so
// there's no duplicate-accumulation concern here and this is just
// Unban() with a presence check first so the caller gets an accurate
// removed count (0 or 1) instead of always reporting 1.
func (b *darwinBanner) RemoveAllRules(ip string) (int, error) {
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
	present := false
	for _, e := range existing {
		if e == ip {
			present = true
			break
		}
	}
	if !present {
		return 0, nil
	}
	if err := b.Unban(ip); err != nil {
		return 0, err
	}
	return 1, nil
}

func (b *darwinBanner) runPfctl(ip string, args ...string) error {
	if !b.available {
		return fmt.Errorf("firewall: pf not available (%s)", b.detail)
	}
	if err := validateIP(ip); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	out, err := b.run(ctx, "pfctl", args...)
	if err != nil {
		return fmt.Errorf("firewall: pfctl %v: %w (output: %s)", args, err, string(out))
	}
	return nil
}
