package geoip

import (
	"os"
	"path/filepath"
	"testing"

	dbmanager "github.com/a-digi/coco-orm/orm"
	_ "github.com/mattn/go-sqlite3"
)

// migration001 mirrors api/config/db/geoip_migrations/001_initial.sql
// exactly (the /***Statement***/ markers are cosmetic — coco-orm's
// executeSQLFile just splits on ";", so they're dropped here without
// changing behavior). Written to a real temp migrations folder rather
// than read via config.ExtractGeoIPMigrationsToTemp(), so this test
// doesn't depend on the working directory a test binary happens to
// run from — same approach dbarchive/archiver_test.go already uses
// for ip_attacks_migrations.
const migration001 = `
CREATE TABLE IF NOT EXISTS geoip_country_ranges
(
    id           INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    family       TEXT NOT NULL,
    start_ip     TEXT NOT NULL,
    end_ip       TEXT NOT NULL,
    country_code TEXT,
    country_name TEXT
);
CREATE INDEX IF NOT EXISTS geoip_country_ranges_lookup_idx ON geoip_country_ranges (family, start_ip DESC);
CREATE TABLE IF NOT EXISTS geoip_asn_ranges
(
    id       INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    family   TEXT NOT NULL,
    start_ip TEXT NOT NULL,
    end_ip   TEXT NOT NULL,
    asn      INTEGER,
    as_org   TEXT
);
CREATE INDEX IF NOT EXISTS geoip_asn_ranges_lookup_idx ON geoip_asn_ranges (family, start_ip DESC);
CREATE TABLE IF NOT EXISTS geoip_meta
(
    key   TEXT NOT NULL PRIMARY KEY,
    value TEXT NOT NULL
);
`

// migration002 mirrors
// api/config/db/geoip_migrations/002_city_enrichment.sql exactly. See
// plan/geoip-enrichment/plan.md's "Extension: city-level GeoIP"
// section.
const migration002 = `
CREATE TABLE IF NOT EXISTS geoip_city_ranges
(
    id           INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    family       TEXT NOT NULL,
    start_ip     TEXT NOT NULL,
    end_ip       TEXT NOT NULL,
    country_code TEXT,
    country_name TEXT,
    subdivision  TEXT,
    city         TEXT,
    postal_code  TEXT,
    latitude     REAL,
    longitude    REAL
);
CREATE INDEX IF NOT EXISTS geoip_city_ranges_lookup_idx ON geoip_city_ranges (family, start_ip DESC);
`

// newTestManager builds a *dbmanager.DatabaseManager against a fresh
// on-disk SQLite file in t.TempDir(), migrated via the real
// SyncMigrations runner against migration001/migration002 above.
func newTestManager(t *testing.T) *dbmanager.DatabaseManager {
	t.Helper()
	root := t.TempDir()

	migrationsDir := filepath.Join(root, "migrations")
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		t.Fatalf("mkdir migrations dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(migrationsDir, "001_initial.sql"), []byte(migration001), 0644); err != nil {
		t.Fatalf("write migration 001: %v", err)
	}
	if err := os.WriteFile(filepath.Join(migrationsDir, "002_city_enrichment.sql"), []byte(migration002), 0644); err != nil {
		t.Fatalf("write migration 002: %v", err)
	}

	manager, err := dbmanager.NewDatabaseManager("geoip-test.db", filepath.Join(root, "db"), []string{migrationsDir})
	if err != nil {
		t.Fatalf("NewDatabaseManager() error = %v", err)
	}
	t.Cleanup(func() { _ = manager.Connector.Close() })

	if err := manager.SyncMigrations(); err != nil {
		t.Fatalf("SyncMigrations() error = %v", err)
	}
	return manager
}

// TestGeoIPMigrations_ApplyCleanlyToAFreshDB exercises the real
// coco-orm migration runner against the geoip_migrations/001_initial.sql
// content, the same way the geoip-updater will build a fresh
// generation before every atomic rename. See
// plan/geoip-enrichment/plan.md step 2.
func TestGeoIPMigrations_ApplyCleanlyToAFreshDB(t *testing.T) {
	manager := newTestManager(t)

	for _, table := range []string{"geoip_country_ranges", "geoip_city_ranges", "geoip_asn_ranges", "geoip_meta"} {
		exists, err := manager.TableExists(table)
		if err != nil {
			t.Fatalf("TableExists(%q) error = %v", table, err)
		}
		if !exists {
			t.Fatalf("table %q was not created by the migration", table)
		}
	}

	db := manager.Connector.DB

	// Column shapes matter here (they're what the importer/SQLLookup
	// will rely on in later steps) — insert+select a representative
	// row through each table to confirm names and types line up.
	if _, err := db.Exec(
		`INSERT INTO geoip_country_ranges (family, start_ip, end_ip, country_code, country_name)
		 VALUES ('v4', '01020300', '010203ff', 'DE', 'Germany')`,
	); err != nil {
		t.Fatalf("insert into geoip_country_ranges: %v", err)
	}
	var countryCode, countryName string
	if err := db.QueryRow(
		`SELECT country_code, country_name FROM geoip_country_ranges WHERE family = 'v4' AND start_ip <= '01020350' ORDER BY start_ip DESC LIMIT 1`,
	).Scan(&countryCode, &countryName); err != nil {
		t.Fatalf("query geoip_country_ranges: %v", err)
	}
	if countryCode != "DE" || countryName != "Germany" {
		t.Fatalf("geoip_country_ranges round-trip = (%q, %q), want (DE, Germany)", countryCode, countryName)
	}

	if _, err := db.Exec(
		`INSERT INTO geoip_city_ranges (family, start_ip, end_ip, country_code, country_name, subdivision, city, postal_code, latitude, longitude)
		 VALUES ('v4', '01020300', '010203ff', 'DE', 'Germany', 'Berlin', 'Berlin', '10115', 52.5244, 13.4105)`,
	); err != nil {
		t.Fatalf("insert into geoip_city_ranges: %v", err)
	}
	var cityCountryCode, city, postalCode string
	var lat, lon float64
	if err := db.QueryRow(
		`SELECT country_code, city, postal_code, latitude, longitude FROM geoip_city_ranges WHERE family = 'v4' AND start_ip <= '01020350' ORDER BY start_ip DESC LIMIT 1`,
	).Scan(&cityCountryCode, &city, &postalCode, &lat, &lon); err != nil {
		t.Fatalf("query geoip_city_ranges: %v", err)
	}
	if cityCountryCode != "DE" || city != "Berlin" || postalCode != "10115" || lat != 52.5244 || lon != 13.4105 {
		t.Fatalf("geoip_city_ranges round-trip = (%q, %q, %q, %v, %v), want (DE, Berlin, 10115, 52.5244, 13.4105)",
			cityCountryCode, city, postalCode, lat, lon)
	}

	if _, err := db.Exec(
		`INSERT INTO geoip_asn_ranges (family, start_ip, end_ip, asn, as_org) VALUES ('v4', '01020300', '010203ff', 3320, 'Deutsche Telekom AG')`,
	); err != nil {
		t.Fatalf("insert into geoip_asn_ranges: %v", err)
	}
	var asn int
	var asOrg string
	if err := db.QueryRow(
		`SELECT asn, as_org FROM geoip_asn_ranges WHERE family = 'v4' AND start_ip <= '01020350' ORDER BY start_ip DESC LIMIT 1`,
	).Scan(&asn, &asOrg); err != nil {
		t.Fatalf("query geoip_asn_ranges: %v", err)
	}
	if asn != 3320 || asOrg != "Deutsche Telekom AG" {
		t.Fatalf("geoip_asn_ranges round-trip = (%d, %q), want (3320, Deutsche Telekom AG)", asn, asOrg)
	}

	if _, err := db.Exec(`INSERT INTO geoip_meta (key, value) VALUES ('last_pulled_at', '2026-07-29T00:00:00Z')`); err != nil {
		t.Fatalf("insert into geoip_meta: %v", err)
	}
	var lastPulledAt string
	if err := db.QueryRow(`SELECT value FROM geoip_meta WHERE key = 'last_pulled_at'`).Scan(&lastPulledAt); err != nil {
		t.Fatalf("query geoip_meta: %v", err)
	}
	if lastPulledAt != "2026-07-29T00:00:00Z" {
		t.Fatalf("geoip_meta round-trip = %q, want 2026-07-29T00:00:00Z", lastPulledAt)
	}
}

// TestGeoIPMigrations_IndexesAllowMultipleRowsPerFamily confirms the
// lookup indexes are non-unique — several distinct blocks for the
// same family must all be insertable; nothing about the index should
// constrain row uniqueness.
func TestGeoIPMigrations_IndexesAllowMultipleRowsPerFamily(t *testing.T) {
	manager := newTestManager(t)
	db := manager.Connector.DB

	for _, start := range []string{"00000000", "01020300", "0a0a0a00"} {
		if _, err := db.Exec(
			`INSERT INTO geoip_country_ranges (family, start_ip, end_ip, country_code, country_name) VALUES ('v4', ?, ?, 'US', 'United States')`,
			start, start,
		); err != nil {
			t.Fatalf("insert geoip_country_ranges start_ip=%s: %v", start, err)
		}
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM geoip_country_ranges`).Scan(&count); err != nil {
		t.Fatalf("count geoip_country_ranges: %v", err)
	}
	if count != 3 {
		t.Fatalf("geoip_country_ranges row count = %d, want 3", count)
	}
}
