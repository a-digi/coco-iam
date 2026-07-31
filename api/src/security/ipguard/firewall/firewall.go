// Package firewall drops (and later un-drops) traffic from a banned
// IP at the OS firewall level, on top of the application-layer 429
// enforcement in ipguard.Authorize. See
// plan/ip-abuse-protection/plan.md section 14.
//
// Platform-specific command syntax lives in per-OS files split by Go
// build tag (firewall_linux.go, etc.) rather than runtime branching —
// this file holds only the shared interface, the no-op fallback, and
// IP validation every backend must apply before building a command.
package firewall

import (
	"fmt"
	"net"
	"time"

	"github.com/a-digi/coco-logger/logger"
)

// Banner bans and unbans an IP at the OS firewall level.
//
// Every implementation MUST validate ip with validateIP before it
// ever reaches a constructed command, and MUST invoke the underlying
// tool via an argument-array exec call (never a shell string) — ip
// originates from a request header (ClientIP), attacker-influenced
// data. Skipping either check turns a rate limiter into a remote
// command-execution hole. See plan section 15.
type Banner interface {
	Ban(ip string, duration time.Duration) error
	Unban(ip string) error
	// Name identifies the backend in use: "iptables", "pf",
	// "windows_firewall", or "none" if Available() is false.
	Name() string
	Available() bool
	// Detail explains why Available() is false. Empty when available.
	Detail() string
	// ListBannedIPs returns the IPs currently blocked at the OS level,
	// read live from the backend — informational only (the ip_bans DB
	// table stays the source of truth for what should be banned). One
	// entry per underlying rule, so a duplicated rule shows up more
	// than once — callers that want counts aggregate this themselves.
	// Returns an empty slice, not an error, when unavailable.
	ListBannedIPs() ([]string, error)
	// RemoveAllRules removes every OS-level rule for ip — there may be
	// more than one, e.g. from repeated Ban() calls for an IP that was
	// already banned, since none of these backends check for an
	// existing rule before inserting a new one. Returns how many were
	// actually removed. Never errors when there was nothing to remove
	// (removed=0, err=nil) or when the backend is unavailable.
	RemoveAllRules(ip string) (removed int, err error)
}

// Detect picks the right backend for the current OS (detectPlatform
// is defined per-OS via build tag) and logs once if nothing usable
// was found. Never returns nil, never errors — a missing firewall
// tool degrades to application-layer-only enforcement (the 429 path
// keeps working regardless); the true/false result is what
// GET /admin/security/status reports.
func Detect(log logger.Logger) Banner {
	b := detectPlatform(log)
	if !b.Available() && log != nil {
		log.Warning("firewall: no OS-level backend available (%s) — falling back to application-layer enforcement only", b.Detail())
	}
	return b
}

// NoopBanner is the fallback when no OS-level backend is available or
// implemented for the current platform. Ban/Unban are no-ops that
// never error — enforcement stays purely at the application layer.
type NoopBanner struct {
	detail string
}

func NewNoopBanner(detail string) *NoopBanner {
	return &NoopBanner{detail: detail}
}

func (n *NoopBanner) Ban(ip string, duration time.Duration) error { return nil }
func (n *NoopBanner) Unban(ip string) error                       { return nil }
func (n *NoopBanner) Name() string                                { return "none" }
func (n *NoopBanner) Available() bool                             { return false }
func (n *NoopBanner) Detail() string                              { return n.detail }
func (n *NoopBanner) ListBannedIPs() ([]string, error)            { return nil, nil }
func (n *NoopBanner) RemoveAllRules(ip string) (int, error)       { return 0, nil }

// validateIP rejects anything that isn't a syntactically valid
// IPv4/IPv6 address — the mandatory gate before ip reaches any
// exec.Command argument list. See this package's doc comment and
// plan section 15.
func validateIP(ip string) error {
	if net.ParseIP(ip) == nil {
		return fmt.Errorf("firewall: invalid ip %q", ip)
	}
	return nil
}
