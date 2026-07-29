package geoip

import (
	"database/sql"
	"testing"
)

func TestEncodeIP_IPv4(t *testing.T) {
	family, encoded, ok := encodeIP("1.2.3.0")
	if !ok {
		t.Fatal("encodeIP() ok = false, want true")
	}
	if family != "v4" {
		t.Errorf("family = %q, want v4", family)
	}
	if encoded != "01020300" {
		t.Errorf("encoded = %q, want 01020300", encoded)
	}
}

func TestEncodeIP_IPv6(t *testing.T) {
	family, encoded, ok := encodeIP("2001:db8::1")
	if !ok {
		t.Fatal("encodeIP() ok = false, want true")
	}
	if family != "v6" {
		t.Errorf("family = %q, want v6", family)
	}
	if len(encoded) != 32 {
		t.Errorf("encoded length = %d, want 32 (16 bytes as hex)", len(encoded))
	}
	if encoded != "20010db8000000000000000000000001" {
		t.Errorf("encoded = %q, want 20010db8000000000000000000000001", encoded)
	}
}

func TestEncodeIP_InvalidReturnsNotOK(t *testing.T) {
	if _, _, ok := encodeIP("not-an-ip"); ok {
		t.Fatal("encodeIP() ok = true for garbage input, want false")
	}
}

// seedCountryRange and seedASNRange insert one range row via a
// (start, end) pair already encoded through encodeIP — the same
// representation the (future) importer will produce.
func seedCountryRange(t *testing.T, db *sql.DB, family, start, end, code, name string) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO geoip_country_ranges (family, start_ip, end_ip, country_code, country_name) VALUES (?, ?, ?, ?, ?)`,
		family, start, end, code, name,
	); err != nil {
		t.Fatalf("seed country range: %v", err)
	}
}

func seedASNRange(t *testing.T, db *sql.DB, family, start, end string, asn int, org string) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO geoip_asn_ranges (family, start_ip, end_ip, asn, as_org) VALUES (?, ?, ?, ?, ?)`,
		family, start, end, asn, org,
	); err != nil {
		t.Fatalf("seed asn range: %v", err)
	}
}

func TestSQLLookup_IPv4Hit(t *testing.T) {
	db := newTestManager(t).Connector.DB
	_, startHex, _ := encodeIP("1.2.3.0")
	_, endHex, _ := encodeIP("1.2.3.255")

	seedCountryRange(t, db, "v4", startHex, endHex, "DE", "Germany")
	seedASNRange(t, db, "v4", startHex, endHex, 3320, "Deutsche Telekom AG")

	lookup := NewSQLLookup(db)
	info, ok := lookup.Lookup("1.2.3.42")
	if !ok {
		t.Fatal("Lookup() ok = false, want true")
	}
	want := Info{CountryCode: "DE", Country: "Germany", ASN: 3320, ASOrg: "Deutsche Telekom AG"}
	if info != want {
		t.Errorf("Lookup() = %+v, want %+v", info, want)
	}
}

func TestSQLLookup_IPv6Hit(t *testing.T) {
	db := newTestManager(t).Connector.DB
	_, startHex, _ := encodeIP("2001:db8::")
	_, endHex, _ := encodeIP("2001:db8::ffff:ffff:ffff:ffff")

	seedCountryRange(t, db, "v6", startHex, endHex, "US", "United States")

	lookup := NewSQLLookup(db)
	info, ok := lookup.Lookup("2001:db8::1")
	if !ok {
		t.Fatal("Lookup() ok = false, want true")
	}
	if info.CountryCode != "US" || info.Country != "United States" {
		t.Errorf("Lookup() = %+v, want country US/United States", info)
	}
	if info.ASN != 0 || info.ASOrg != "" {
		t.Errorf("Lookup() = %+v, want zero ASN fields (no asn range seeded)", info)
	}
}

func TestSQLLookup_MissInAllocationGap(t *testing.T) {
	db := newTestManager(t).Connector.DB
	_, startHex, _ := encodeIP("1.2.3.0")
	_, endHex, _ := encodeIP("1.2.3.255")
	seedCountryRange(t, db, "v4", startHex, endHex, "DE", "Germany")

	lookup := NewSQLLookup(db)
	// 1.2.4.1 sorts after the seeded block's start_ip but falls past
	// its end_ip — nearest-start-below finds the block, but the
	// end_ip check must reject it as not actually covering this IP.
	info, ok := lookup.Lookup("1.2.4.1")
	if ok {
		t.Fatalf("Lookup() ok = true, want false for an IP in an unallocated gap (got %+v)", info)
	}
	if info != (Info{}) {
		t.Errorf("Lookup() info = %+v, want zero value on a miss", info)
	}
}

func TestSQLLookup_MissBelowAnySeededRange(t *testing.T) {
	db := newTestManager(t).Connector.DB
	_, startHex, _ := encodeIP("10.0.0.0")
	_, endHex, _ := encodeIP("10.0.0.255")
	seedCountryRange(t, db, "v4", startHex, endHex, "DE", "Germany")

	lookup := NewSQLLookup(db)
	// No range starts at or below 1.1.1.1 at all.
	if _, ok := lookup.Lookup("1.1.1.1"); ok {
		t.Fatal("Lookup() ok = true, want false when ip sorts before every seeded range")
	}
}

func TestSQLLookup_InvalidIPReturnsNotFound(t *testing.T) {
	db := newTestManager(t).Connector.DB
	lookup := NewSQLLookup(db)
	if _, ok := lookup.Lookup("not-an-ip"); ok {
		t.Fatal("Lookup() ok = true for garbage input, want false")
	}
}

func TestSQLLookup_PartialData_CountryOnlyStillFound(t *testing.T) {
	db := newTestManager(t).Connector.DB
	_, startHex, _ := encodeIP("1.2.3.0")
	_, endHex, _ := encodeIP("1.2.3.255")
	seedCountryRange(t, db, "v4", startHex, endHex, "DE", "Germany")
	// deliberately no ASN range seeded

	lookup := NewSQLLookup(db)
	info, ok := lookup.Lookup("1.2.3.1")
	if !ok {
		t.Fatal("Lookup() ok = false, want true (country alone should be enough)")
	}
	if info.CountryCode != "DE" || info.ASN != 0 {
		t.Errorf("Lookup() = %+v, want country DE and zero ASN", info)
	}
}

func TestSQLLookup_NilDBReturnsNotFound(t *testing.T) {
	lookup := NewSQLLookup(nil)
	if _, ok := lookup.Lookup("203.0.113.7"); ok {
		t.Fatal("Lookup() ok = true with no connection, want false")
	}
}

func TestSQLLookup_EnabledIsAlwaysTrueRegardlessOfConnection(t *testing.T) {
	if !NewSQLLookup(nil).Enabled() {
		t.Fatal("SQLLookup.Enabled() = false, want true even with a nil connection (see Enabled's own doc comment)")
	}
}
