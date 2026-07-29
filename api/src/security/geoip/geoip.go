// Package geoip provides IP -> country/ASN/ISP lookups, backed by a
// locally-maintained SQLite database (geoip.db) built from MaxMind
// GeoLite2 CSV data by the separate geoip-updater executable. This
// package supplies the read side (Lookup, SQLLookup, Watcher) and the
// vocabulary (Info, Config) both processes share; the write side
// lives in api/src/security/geoip/updater. See
// plan/geoip-enrichment/plan.md.
package geoip

import (
	"database/sql"
	"encoding/hex"
	"net"
)

// Info is a snapshot of what's known about one IP's owner/location at
// lookup time. Callers that persist a lookup result (e.g.
// IPGuardSecurityLayer.recordAttackHit, scanwatch.Watcher.RecordHit)
// json.Marshal this directly into an episode's geoip_info column —
// see plan/geoip-enrichment/plan.md's "JSON-not-join" design: a stored
// Info must never be re-derived from a live lookup later, since
// geoip.db keeps no history and could show different (or no) data by
// the time anyone looks back at an old episode.
type Info struct {
	CountryCode string `json:"country_code,omitempty"`
	Country     string `json:"country,omitempty"`
	// Subdivision, City, PostalCode, Latitude, and Longitude come from
	// the same GeoLite2-City dataset Country/CountryCode now do (City
	// is a strict superset of the old Country-only edition — see
	// plan/geoip-enrichment/plan.md's "Extension: city-level GeoIP"
	// section). omitempty on Latitude/Longitude means a genuine 0,0
	// coordinate is indistinguishable from "not set" — acceptable
	// here, since MaxMind itself uses 0,0 as its own "unknown
	// location" placeholder.
	Subdivision string  `json:"subdivision,omitempty"`
	City        string  `json:"city,omitempty"`
	PostalCode  string  `json:"postal_code,omitempty"`
	Latitude    float64 `json:"latitude,omitempty"`
	Longitude   float64 `json:"longitude,omitempty"`
	ASN         uint    `json:"asn,omitempty"`
	ASOrg       string  `json:"as_org,omitempty"`
}

// Lookup resolves an IP to whatever ownership/location info is
// available. Every caller holds a Lookup unconditionally, never nil —
// NoopLookup is the always-present zero-cost implementation when
// geoip is disabled, mirroring the nil-safe conventions already used
// elsewhere in this package tree (e.g. ipguard's firewall.Banner).
type Lookup interface {
	// Lookup returns info and true if ip resolved to something; false
	// if geoip is disabled, or ip has no matching data (e.g. it falls
	// in an unallocated gap).
	Lookup(ip string) (Info, bool)
	// Enabled reports whether this Lookup can ever return real data —
	// false for NoopLookup, true for a real SQLLookup regardless of
	// whether any individual IP resolves.
	Enabled() bool
}

// NoopLookup is the always-present, always-empty Lookup — used when
// geoip is disabled in config, or before a real Lookup has been
// constructed. Callers never need a nil check.
type NoopLookup struct{}

func (NoopLookup) Lookup(ip string) (Info, bool) { return Info{}, false }
func (NoopLookup) Enabled() bool                 { return false }

var _ Lookup = NoopLookup{}

// SQLLookup is the real Lookup implementation, querying geoip.db
// through a swappable connection slot — see dbslot.go and (once
// wired) watcher.go, which swaps the slot's connection whenever the
// geoip-updater executable replaces the file on disk.
type SQLLookup struct {
	slot *dbSlot
}

// NewSQLLookup wraps db (which may be nil — e.g. before Watcher's
// first successful open) as the initial connection.
func NewSQLLookup(db *sql.DB) *SQLLookup {
	return &SQLLookup{slot: newDBSlot(db)}
}

var _ Lookup = (*SQLLookup)(nil)

// Enabled always reports true for a real SQLLookup — it is a
// type-level capability flag ("this is a real lookup, not a no-op"),
// not a runtime-availability flag. A momentarily-nil connection (no
// geoip.db opened yet) is reflected by Lookup returning false, not by
// Enabled.
func (l *SQLLookup) Enabled() bool { return true }

// Lookup queries geoip_city_ranges and geoip_asn_ranges
// independently — city/country and ASN data can each be present or
// absent on their own (GeoLite2 ships them as separate editions), so
// a miss in one must not suppress a hit in the other. Returns false
// only if neither query found anything (or geoip.db isn't open yet,
// or ip doesn't parse).
func (l *SQLLookup) Lookup(ip string) (Info, bool) {
	db := l.slot.DB()
	if db == nil {
		return Info{}, false
	}
	family, encoded, ok := encodeIP(ip)
	if !ok {
		return Info{}, false
	}

	var info Info
	found := false

	if city, ok := lookupCity(db, family, encoded); ok {
		info = city
		found = true
	}
	if asn, asOrg, ok := lookupASN(db, family, encoded); ok {
		info.ASN = asn
		info.ASOrg = asOrg
		found = true
	}

	return info, found
}

// lookupCity finds the city range whose start_ip is the largest one
// not exceeding encoded, then confirms encoded actually falls within
// that range (end_ip >= encoded) — the standard "nearest-start-below,
// then confirm coverage" technique for CIDR range lookups, using the
// (family, start_ip DESC) index for an O(log n) seek instead of a
// full range scan. A false result here means either no block covers
// encoded (an unallocated gap) or the query itself failed — both are
// legitimate "no data" outcomes for this enrichment feature, never
// worth surfacing as an error.
func lookupCity(db *sql.DB, family, encoded string) (Info, bool) {
	var startIP, endIP string
	var cc, country, subdivision, city, postal sql.NullString
	var lat, lon sql.NullFloat64
	err := db.QueryRow(
		`SELECT start_ip, end_ip, country_code, country_name, subdivision, city, postal_code, latitude, longitude
		 FROM geoip_city_ranges WHERE family = ? AND start_ip <= ? ORDER BY start_ip DESC LIMIT 1`,
		family, encoded,
	).Scan(&startIP, &endIP, &cc, &country, &subdivision, &city, &postal, &lat, &lon)
	if err != nil || endIP < encoded {
		return Info{}, false
	}
	return Info{
		CountryCode: cc.String,
		Country:     country.String,
		Subdivision: subdivision.String,
		City:        city.String,
		PostalCode:  postal.String,
		Latitude:    lat.Float64,
		Longitude:   lon.Float64,
	}, true
}

// lookupASN mirrors lookupCountry against geoip_asn_ranges.
func lookupASN(db *sql.DB, family, encoded string) (asn uint, asOrg string, ok bool) {
	var startIP, endIP string
	var asnVal sql.NullInt64
	var org sql.NullString
	err := db.QueryRow(
		`SELECT start_ip, end_ip, asn, as_org FROM geoip_asn_ranges
		 WHERE family = ? AND start_ip <= ? ORDER BY start_ip DESC LIMIT 1`,
		family, encoded,
	).Scan(&startIP, &endIP, &asnVal, &org)
	if err != nil || endIP < encoded {
		return 0, "", false
	}
	return uint(asnVal.Int64), org.String, true
}

// encodeIP is EncodeIP for a string-form address (what Lookup
// receives from callers) — parses then delegates.
func encodeIP(ip string) (family, encoded string, ok bool) {
	return EncodeIP(net.ParseIP(ip))
}

// EncodeIP returns the family ("v4" or "v6") and fixed-width,
// zero-padded hex representation of ip, matching exactly how
// start_ip/end_ip are stored in geoip_country_ranges/geoip_asn_ranges
// — hex.EncodeToString on a fixed-size 4-byte or 16-byte slice always
// produces exactly 8 or 32 hex characters, zero-padded by
// construction, so lexicographic string comparison equals numeric
// comparison. ok is false if ip is nil or doesn't reduce to a 4- or
// 16-byte form.
//
// Exported so the separate geoip/updater package (which builds these
// tables from MaxMind CIDR data) encodes rows in exact agreement with
// how SQLLookup reads them back — the two must never drift out of
// sync with each other.
func EncodeIP(ip net.IP) (family, encoded string, ok bool) {
	if ip == nil {
		return "", "", false
	}
	if v4 := ip.To4(); v4 != nil {
		return "v4", hex.EncodeToString(v4), true
	}
	v6 := ip.To16()
	if v6 == nil {
		return "", "", false
	}
	return "v6", hex.EncodeToString(v6), true
}

// IsLoopbackOrPrivate reports whether ip parses as a loopback
// (127.0.0.0/8, ::1) or RFC1918/ULA private address. Shared by every
// caller (ipguard, scanwatch) that needs to decide whether an address
// is worth a geoip lookup at all, and — inverted — whether it's
// worth capturing an origin-hint diagnostic snapshot instead (see
// plan/attack-ip-attribution/plan.md Fix 3): a genuine public
// attacker IP gets geoip data and no origin hint; a loopback/private
// result gets no geoip data (meaningless for a private range) but is
// exactly the case an origin hint is useful for.
func IsLoopbackOrPrivate(ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	return parsed.IsLoopback() || parsed.IsPrivate()
}
