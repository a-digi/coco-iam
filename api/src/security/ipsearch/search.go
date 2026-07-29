// Package ipsearch answers "search for this IP" queries by composing a
// live geoip.Lookup with the distinct known IPs already recorded in
// ip-attacks.db (ip_attacks/scan_episodes). Deliberately its own
// package rather than folded into api/src/security/geoip: it composes
// two domains this plan otherwise keeps decoupled (geoip.db lookups
// and attack/scan history), while still living under api/src/security
// per this whole plan's established placement. See
// plan/geoip-enrichment/plan.md's "Extension: IP search" section.
package ipsearch

import (
	"fmt"
	"net"
	"strings"

	"github.com/a-digi/coco-iam/src/security/dbhandle"
	"github.com/a-digi/coco-iam/src/security/geoip"
)

// DefaultLimit and MaxLimit bound how many autocomplete suggestions a
// partial-prefix search returns — Search clamps to this range
// regardless of what's requested, so a caller can't force an
// unbounded number of live geoip lookups in one call.
const (
	DefaultLimit = 10
	MaxLimit     = 25
)

// Result is one IP's live geoip lookup outcome. Matched is false when
// geoip.db has no coverage for it (loopback/private, or simply no
// GeoLite2 allocation data for that address) — a normal, expected
// outcome, not an error, mirroring geoip.Lookup's own convention.
type Result struct {
	IP          string
	Matched     bool
	CountryCode string
	Country     string
	Subdivision string
	City        string
	PostalCode  string
	ASN         uint
	ASOrg       string
}

// Searcher answers IP search queries. Both dependencies are already
// constructed elsewhere (ContextBag.GetGeoIP()/GetIPAttacksHandle()) —
// this is a cheap, stateless composition, not a new owned resource.
type Searcher struct {
	geo    geoip.Lookup
	handle *dbhandle.Handle
}

func NewSearcher(geo geoip.Lookup, handle *dbhandle.Handle) *Searcher {
	return &Searcher{geo: geo, handle: handle}
}

// Search returns geoip results for q. A complete, valid IP (net.ParseIP
// succeeds) yields exactly one result for that address, via a live
// lookup — this works for any valid IP, not just ones this server has
// ever recorded. An incomplete/partial q yields up to limit known IPs
// from ip_attacks/scan_episodes history starting with that prefix,
// each resolved through that same live lookup rather than their
// frozen geoip_info snapshot, so both branches reflect the current
// geoip.db consistently.
func (s *Searcher) Search(q string, limit int) ([]Result, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return nil, fmt.Errorf("ipsearch: q must not be empty")
	}
	limit = clampLimit(limit)

	if net.ParseIP(q) != nil {
		return []Result{s.lookup(q)}, nil
	}

	ips, err := s.suggestKnownIPs(q, limit)
	if err != nil {
		return nil, err
	}
	out := make([]Result, 0, len(ips))
	for _, ip := range ips {
		out = append(out, s.lookup(ip))
	}
	return out, nil
}

func clampLimit(limit int) int {
	if limit <= 0 {
		return DefaultLimit
	}
	if limit > MaxLimit {
		return MaxLimit
	}
	return limit
}

func (s *Searcher) lookup(ip string) Result {
	info, ok := s.geo.Lookup(ip)
	return Result{
		IP:          ip,
		Matched:     ok,
		CountryCode: info.CountryCode,
		Country:     info.Country,
		Subdivision: info.Subdivision,
		City:        info.City,
		PostalCode:  info.PostalCode,
		ASN:         info.ASN,
		ASOrg:       info.ASOrg,
	}
}

// suggestKnownIPs returns distinct IPs from ip_attacks/scan_episodes
// starting with prefix (plain UNION already dedupes across the two
// tables). prefix is escaped for SQL LIKE wildcards first — a literal
// `%`/`_` an admin pastes should match literally, not widen the
// search; the query stays fully parameterized regardless, so this is
// a correctness fix, not a SQL-injection concern.
func (s *Searcher) suggestKnownIPs(prefix string, limit int) ([]string, error) {
	pattern := escapeLike(prefix) + "%"
	rows, err := s.handle.DB().Query(
		`SELECT ip FROM (
			SELECT ip FROM ip_attacks WHERE ip LIKE ? ESCAPE '\'
			UNION
			SELECT ip FROM scan_episodes WHERE ip LIKE ? ESCAPE '\'
		) ORDER BY ip LIMIT ?`,
		pattern, pattern, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("ipsearch: suggest known ips: %w", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var ip string
		if err := rows.Scan(&ip); err != nil {
			return nil, fmt.Errorf("ipsearch: scan: %w", err)
		}
		out = append(out, ip)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ipsearch: rows: %w", err)
	}
	return out, nil
}

func escapeLike(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}
