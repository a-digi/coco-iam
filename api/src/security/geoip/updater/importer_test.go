package updater

import (
	"archive/zip"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// geoipSchema mirrors api/config/db/geoip_migrations/001_initial.sql
// exactly (the /***Statement***/ markers are cosmetic, coco-orm's
// migration runner just splits on ";") — inlined here rather than
// read from disk so these tests don't depend on the working directory
// a test binary happens to run from. Same approach
// geoip/schema_test.go and dbarchive/archiver_test.go already use.
const geoipSchema = `
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

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "geoip-test.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(geoipSchema); err != nil {
		t.Fatalf("apply schema: %v", err)
	}
	return db
}

func TestCidrRange_IPv4(t *testing.T) {
	start, end, err := cidrRange("1.2.3.0/24")
	if err != nil {
		t.Fatalf("cidrRange() error = %v", err)
	}
	if start.String() != "1.2.3.0" {
		t.Errorf("start = %v, want 1.2.3.0", start)
	}
	if end.String() != "1.2.3.255" {
		t.Errorf("end = %v, want 1.2.3.255", end)
	}
}

func TestCidrRange_IPv6(t *testing.T) {
	start, end, err := cidrRange("2001:db8::/32")
	if err != nil {
		t.Fatalf("cidrRange() error = %v", err)
	}
	if start.String() != "2001:db8::" {
		t.Errorf("start = %v, want 2001:db8::", start)
	}
	if end.String() != "2001:db8:ffff:ffff:ffff:ffff:ffff:ffff" {
		t.Errorf("end = %v, want 2001:db8:ffff:ffff:ffff:ffff:ffff:ffff", end)
	}
}

func TestCidrRange_InvalidReturnsError(t *testing.T) {
	if _, _, err := cidrRange("not-a-cidr"); err == nil {
		t.Fatal("cidrRange() error = nil, want an error for garbage input")
	}
}

func TestParseCountryLocations_ParsesRows(t *testing.T) {
	csv := "geoname_id,locale_code,continent_code,continent_name,country_iso_code,country_name,is_in_european_union\n" +
		"2921044,en,EU,Europe,DE,Germany,1\n" +
		"6252001,en,NA,North America,US,United States,0\n"

	locations, err := parseCountryLocations(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("parseCountryLocations() error = %v", err)
	}
	if len(locations) != 2 {
		t.Fatalf("locations = %+v, want 2 entries", locations)
	}
	if got := locations["2921044"]; got.code != "DE" || got.name != "Germany" {
		t.Errorf("locations[2921044] = %+v, want {DE Germany}", got)
	}
	if got := locations["6252001"]; got.code != "US" || got.name != "United States" {
		t.Errorf("locations[6252001] = %+v, want {US United States}", got)
	}
}

func TestImportCountryBlocks_JoinsLocationsAndInsertsRows(t *testing.T) {
	db := newTestDB(t)
	locations := map[string]countryInfo{
		"2921044": {code: "DE", name: "Germany"},
	}
	blocksCSV := "network,geoname_id,registered_country_geoname_id,represented_country_geoname_id,is_anonymous_proxy,is_satellite_provider\n" +
		"1.2.3.0/24,2921044,2921044,,0,0\n"

	n, err := importCountryBlocks(db, "v4", locations, strings.NewReader(blocksCSV))
	if err != nil {
		t.Fatalf("importCountryBlocks() error = %v", err)
	}
	if n != 1 {
		t.Fatalf("importCountryBlocks() returned %d, want 1", n)
	}

	var family, startIP, endIP, code, name string
	if err := db.QueryRow(`SELECT family, start_ip, end_ip, country_code, country_name FROM geoip_country_ranges`).
		Scan(&family, &startIP, &endIP, &code, &name); err != nil {
		t.Fatalf("query inserted row: %v", err)
	}
	if family != "v4" || code != "DE" || name != "Germany" {
		t.Errorf("inserted row = (family=%s code=%s name=%s), want (v4 DE Germany)", family, code, name)
	}
	if startIP != "01020300" || endIP != "010203ff" {
		t.Errorf("inserted range = (%s, %s), want (01020300, 010203ff)", startIP, endIP)
	}
}

func TestImportCountryBlocks_FallsBackToRegisteredCountryWhenGeonameBlank(t *testing.T) {
	db := newTestDB(t)
	locations := map[string]countryInfo{
		"6252001": {code: "US", name: "United States"},
	}
	// geoname_id blank (e.g. an anonymizing-proxy block), but
	// registered_country_geoname_id still points at a real country.
	blocksCSV := "network,geoname_id,registered_country_geoname_id,represented_country_geoname_id,is_anonymous_proxy,is_satellite_provider\n" +
		"9.9.9.0/24,,6252001,,1,0\n"

	n, err := importCountryBlocks(db, "v4", locations, strings.NewReader(blocksCSV))
	if err != nil {
		t.Fatalf("importCountryBlocks() error = %v", err)
	}
	if n != 1 {
		t.Fatalf("importCountryBlocks() returned %d, want 1", n)
	}

	var code, name string
	if err := db.QueryRow(`SELECT country_code, country_name FROM geoip_country_ranges`).Scan(&code, &name); err != nil {
		t.Fatalf("query inserted row: %v", err)
	}
	if code != "US" || name != "United States" {
		t.Errorf("inserted row = (code=%s name=%s), want (US United States) via registered_country_geoname_id fallback", code, name)
	}
}

func TestImportCountryBlocks_SkipsUnparseableNetworkRatherThanAborting(t *testing.T) {
	db := newTestDB(t)
	locations := map[string]countryInfo{"1": {code: "DE", name: "Germany"}}
	blocksCSV := "network,geoname_id,registered_country_geoname_id,represented_country_geoname_id,is_anonymous_proxy,is_satellite_provider\n" +
		"not-a-network,1,1,,0,0\n" +
		"1.2.3.0/24,1,1,,0,0\n"

	n, err := importCountryBlocks(db, "v4", locations, strings.NewReader(blocksCSV))
	if err != nil {
		t.Fatalf("importCountryBlocks() error = %v", err)
	}
	if n != 1 {
		t.Fatalf("importCountryBlocks() returned %d, want 1 (the bad row should be skipped, not fatal)", n)
	}
}

func TestImportASNBlocks_InsertsRows(t *testing.T) {
	db := newTestDB(t)
	blocksCSV := "network,autonomous_system_number,autonomous_system_organization\n" +
		"1.2.3.0/24,3320,Deutsche Telekom AG\n"

	n, err := importASNBlocks(db, "v4", strings.NewReader(blocksCSV))
	if err != nil {
		t.Fatalf("importASNBlocks() error = %v", err)
	}
	if n != 1 {
		t.Fatalf("importASNBlocks() returned %d, want 1", n)
	}

	var family string
	var asn int
	var org string
	if err := db.QueryRow(`SELECT family, asn, as_org FROM geoip_asn_ranges`).Scan(&family, &asn, &org); err != nil {
		t.Fatalf("query inserted row: %v", err)
	}
	if family != "v4" || asn != 3320 || org != "Deutsche Telekom AG" {
		t.Errorf("inserted row = (family=%s asn=%d org=%s), want (v4 3320 Deutsche Telekom AG)", family, asn, org)
	}
}

func TestImportASNBlocks_SkipsUnparseableASNRatherThanAborting(t *testing.T) {
	db := newTestDB(t)
	blocksCSV := "network,autonomous_system_number,autonomous_system_organization\n" +
		"1.2.3.0/24,not-a-number,Bad Org\n" +
		"9.9.9.0/24,64512,Good Org\n"

	n, err := importASNBlocks(db, "v4", strings.NewReader(blocksCSV))
	if err != nil {
		t.Fatalf("importASNBlocks() error = %v", err)
	}
	if n != 1 {
		t.Fatalf("importASNBlocks() returned %d, want 1 (the bad row should be skipped, not fatal)", n)
	}
}

func TestFindCSVFile_FindsNestedFile(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "GeoLite2-Country-CSV_20260101")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	target := filepath.Join(nested, "GeoLite2-Country-Locations-en.csv")
	if err := os.WriteFile(target, []byte("geoname_id\n"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	found, err := findCSVFile(root, "GeoLite2-Country-Locations-en.csv")
	if err != nil {
		t.Fatalf("findCSVFile() error = %v", err)
	}
	if found != target {
		t.Errorf("findCSVFile() = %q, want %q", found, target)
	}
}

func TestFindCSVFile_NotFoundIsAnError(t *testing.T) {
	root := t.TempDir()
	if _, err := findCSVFile(root, "does-not-exist.csv"); err == nil {
		t.Fatal("findCSVFile() error = nil, want an error when the file isn't present")
	}
}

// buildTestZip writes a zip archive at destPath containing the given
// name -> content entries, optionally nested under dirPrefix (mirrors
// MaxMind's own versioned-subdirectory wrapping).
func buildTestZip(t *testing.T, destPath, dirPrefix string, files map[string]string) {
	t.Helper()
	out, err := os.Create(destPath)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	defer out.Close()

	w := zip.NewWriter(out)
	for name, content := range files {
		entryName := name
		if dirPrefix != "" {
			entryName = dirPrefix + "/" + name
		}
		f, err := w.Create(entryName)
		if err != nil {
			t.Fatalf("create zip entry %s: %v", entryName, err)
		}
		if _, err := f.Write([]byte(content)); err != nil {
			t.Fatalf("write zip entry %s: %v", entryName, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
}

func TestUnzip_ExtractsNestedFiles(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "test.zip")
	buildTestZip(t, zipPath, "GeoLite2-Country-CSV_20260101", map[string]string{
		"GeoLite2-Country-Locations-en.csv": "geoname_id\n",
	})

	destDir := filepath.Join(dir, "extracted")
	if err := unzip(zipPath, destDir); err != nil {
		t.Fatalf("unzip() error = %v", err)
	}

	found, err := findCSVFile(destDir, "GeoLite2-Country-Locations-en.csv")
	if err != nil {
		t.Fatalf("findCSVFile() after unzip error = %v", err)
	}
	content, err := os.ReadFile(found)
	if err != nil {
		t.Fatalf("read extracted file: %v", err)
	}
	if string(content) != "geoname_id\n" {
		t.Errorf("extracted content = %q, want %q", content, "geoname_id\n")
	}
}

func TestUnzip_RejectsPathTraversal(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "malicious.zip")

	out, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	w := zip.NewWriter(out)
	f, err := w.Create("../../etc/passwd")
	if err != nil {
		t.Fatalf("create zip entry: %v", err)
	}
	if _, err := f.Write([]byte("pwned")); err != nil {
		t.Fatalf("write zip entry: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	_ = out.Close()

	destDir := filepath.Join(dir, "extracted")
	if err := unzip(zipPath, destDir); err == nil {
		t.Fatal("unzip() error = nil, want an error for a path-traversal zip entry")
	}
}
