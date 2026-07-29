package updater

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/a-digi/coco-iam/src/security/geoip"
)

// newTestMigrationsDir writes geoipSchema (see importer_test.go) to a
// temp migrations folder, for constructing an Updater without any
// dependency on api/config's working-directory-relative path
// resolution — same approach every other test file in this codebase
// uses for a real migrations folder.
func newTestMigrationsDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "001_initial.sql"), []byte(geoipSchema), 0644); err != nil {
		t.Fatalf("write migration: %v", err)
	}
	return dir
}

// fixtureServer serves small, hand-built country/ASN CSV zips at the
// real MaxMind download URL shape, tracking how many requests it
// received.
func fixtureServer(t *testing.T, countryZipPath, asnZipPath string) (*httptest.Server, *int32) {
	t.Helper()
	var requestCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		switch {
		case strings.HasPrefix(r.URL.Path, "/"+editionCountryCSV+"/"):
			http.ServeFile(w, r, countryZipPath)
		case strings.HasPrefix(r.URL.Path, "/"+editionASNCSV+"/"):
			http.ServeFile(w, r, asnZipPath)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	return srv, &requestCount
}

func buildFixtureZips(t *testing.T) (countryZipPath, asnZipPath string) {
	t.Helper()
	dir := t.TempDir()

	countryZipPath = filepath.Join(dir, "country.zip")
	buildTestZip(t, countryZipPath, "GeoLite2-Country-CSV_20260101", map[string]string{
		"GeoLite2-Country-Locations-en.csv": "geoname_id,locale_code,continent_code,continent_name,country_iso_code,country_name,is_in_european_union\n" +
			"2921044,en,EU,Europe,DE,Germany,1\n",
		"GeoLite2-Country-Blocks-IPv4.csv": "network,geoname_id,registered_country_geoname_id,represented_country_geoname_id,is_anonymous_proxy,is_satellite_provider\n" +
			"1.2.3.0/24,2921044,2921044,,0,0\n",
		"GeoLite2-Country-Blocks-IPv6.csv": "network,geoname_id,registered_country_geoname_id,represented_country_geoname_id,is_anonymous_proxy,is_satellite_provider\n",
	})

	asnZipPath = filepath.Join(dir, "asn.zip")
	buildTestZip(t, asnZipPath, "GeoLite2-ASN-CSV_20260101", map[string]string{
		"GeoLite2-ASN-Blocks-IPv4.csv": "network,autonomous_system_number,autonomous_system_organization\n" +
			"1.2.3.0/24,3320,Deutsche Telekom AG\n",
		"GeoLite2-ASN-Blocks-IPv6.csv": "network,autonomous_system_number,autonomous_system_organization\n",
	})

	return countryZipPath, asnZipPath
}

func testConfig(dbPath string) geoip.Config {
	return geoip.Config{
		Enabled:              true,
		DBPath:               dbPath,
		CheckIntervalSeconds: 600,
		PullIntervalHours:    24,
		MaxMindAccountID:     "acct-1",
		MaxMindLicenseKey:    "key-1",
	}
}

func TestUpdater_PullAndImport_EndToEnd(t *testing.T) {
	countryZipPath, asnZipPath := buildFixtureZips(t)
	srv, _ := fixtureServer(t, countryZipPath, asnZipPath)
	defer srv.Close()

	dbPath := filepath.Join(t.TempDir(), "geoip.db")
	cfg := testConfig(dbPath)
	u := New(cfg, newTestMigrationsDir(t), nil)
	u.downloader.baseURL = srv.URL

	if err := u.pullAndImport(context.Background()); err != nil {
		t.Fatalf("pullAndImport() error = %v", err)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open resulting geoip.db: %v", err)
	}
	defer db.Close()

	lookup := geoip.NewSQLLookup(db)
	info, ok := lookup.Lookup("1.2.3.42")
	if !ok {
		t.Fatal("Lookup() ok = false, want true against the freshly imported db")
	}
	want := geoip.Info{CountryCode: "DE", Country: "Germany", ASN: 3320, ASOrg: "Deutsche Telekom AG"}
	if info != want {
		t.Errorf("Lookup() = %+v, want %+v", info, want)
	}

	var lastPulledAt string
	if err := db.QueryRow(`SELECT value FROM geoip_meta WHERE key = 'last_pulled_at'`).Scan(&lastPulledAt); err != nil {
		t.Fatalf("query last_pulled_at: %v", err)
	}
	if _, err := time.Parse(time.RFC3339, lastPulledAt); err != nil {
		t.Errorf("last_pulled_at = %q, want a valid RFC3339 timestamp: %v", lastPulledAt, err)
	}

	// The temp "*.building-<uuid>" file must not linger next to the
	// canonical path after a successful rename.
	entries, err := os.ReadDir(filepath.Dir(dbPath))
	if err != nil {
		t.Fatalf("read db dir: %v", err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".building-") {
			t.Errorf("leftover build artifact %q found after a successful pull", e.Name())
		}
	}
}

func TestUpdater_TryPull_SkipsWhenRecentEnough(t *testing.T) {
	countryZipPath, asnZipPath := buildFixtureZips(t)
	srv, requestCount := fixtureServer(t, countryZipPath, asnZipPath)
	defer srv.Close()

	dbPath := filepath.Join(t.TempDir(), "geoip.db")
	seedDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open seed db: %v", err)
	}
	if _, err := seedDB.Exec(geoipSchema); err != nil {
		t.Fatalf("apply schema: %v", err)
	}
	if _, err := seedDB.Exec(
		`INSERT INTO geoip_meta (key, value) VALUES ('last_pulled_at', ?)`,
		time.Now().UTC().Format(time.RFC3339),
	); err != nil {
		t.Fatalf("seed last_pulled_at: %v", err)
	}
	if err := seedDB.Close(); err != nil {
		t.Fatalf("close seed db: %v", err)
	}

	cfg := testConfig(dbPath)
	u := New(cfg, newTestMigrationsDir(t), nil)
	u.downloader.baseURL = srv.URL

	u.tryPull(context.Background())

	if got := atomic.LoadInt32(requestCount); got != 0 {
		t.Fatalf("tryPull() made %d HTTP request(s), want 0 (last pull was recent enough)", got)
	}
}

func TestUpdater_TryPull_PullsWhenNeverPulledBefore(t *testing.T) {
	countryZipPath, asnZipPath := buildFixtureZips(t)
	srv, requestCount := fixtureServer(t, countryZipPath, asnZipPath)
	defer srv.Close()

	dbPath := filepath.Join(t.TempDir(), "geoip.db") // never created
	cfg := testConfig(dbPath)
	u := New(cfg, newTestMigrationsDir(t), nil)
	u.downloader.baseURL = srv.URL

	u.tryPull(context.Background())

	if got := atomic.LoadInt32(requestCount); got != 2 {
		t.Fatalf("tryPull() made %d HTTP request(s), want 2 (country + asn editions)", got)
	}
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("geoip.db was not created: %v", err)
	}
}

func TestUpdater_SanityCheck_RejectsEmptyCountryData(t *testing.T) {
	u := &Updater{cfg: testConfig(filepath.Join(t.TempDir(), "geoip.db"))}
	if err := u.sanityCheck(0, 100); err == nil {
		t.Fatal("sanityCheck() error = nil, want an error for zero country ranges")
	}
}

func TestUpdater_SanityCheck_RejectsEmptyASNData(t *testing.T) {
	u := &Updater{cfg: testConfig(filepath.Join(t.TempDir(), "geoip.db"))}
	if err := u.sanityCheck(100, 0); err == nil {
		t.Fatal("sanityCheck() error = nil, want an error for zero asn ranges")
	}
}

func TestUpdater_SanityCheck_AllowsFirstEverPullRegardlessOfSize(t *testing.T) {
	// No live file exists yet at DBPath — nothing to compare against.
	u := &Updater{cfg: testConfig(filepath.Join(t.TempDir(), "geoip.db"))}
	if err := u.sanityCheck(1, 1); err != nil {
		t.Fatalf("sanityCheck() error = %v, want nil for a first-ever pull", err)
	}
}

func seedRangeCounts(t *testing.T, dbPath string, n int) {
	t.Helper()
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(geoipSchema); err != nil {
		t.Fatalf("apply schema: %v", err)
	}
	for i := 0; i < n; i++ {
		hex := fmt.Sprintf("%08x", i)
		if _, err := db.Exec(`INSERT INTO geoip_country_ranges (family, start_ip, end_ip, country_code, country_name) VALUES ('v4', ?, ?, 'US', 'United States')`, hex, hex); err != nil {
			t.Fatalf("seed country row: %v", err)
		}
		if _, err := db.Exec(`INSERT INTO geoip_asn_ranges (family, start_ip, end_ip, asn, as_org) VALUES ('v4', ?, ?, 1, 'Org')`, hex, hex); err != nil {
			t.Fatalf("seed asn row: %v", err)
		}
	}
}

func TestUpdater_SanityCheck_RejectsSuspiciousDropInCountryData(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "geoip.db")
	seedRangeCounts(t, dbPath, 100)

	u := &Updater{cfg: testConfig(dbPath)}
	// 50 is below 80% of the previous 100 — must fail.
	if err := u.sanityCheck(50, 100); err == nil {
		t.Fatal("sanityCheck() error = nil, want an error for a suspicious drop in country ranges")
	}
}

func TestUpdater_SanityCheck_AllowsSmallButAcceptableDrop(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "geoip.db")
	seedRangeCounts(t, dbPath, 100)

	u := &Updater{cfg: testConfig(dbPath)}
	// 85 is above the 80% floor — must pass.
	if err := u.sanityCheck(85, 100); err != nil {
		t.Fatalf("sanityCheck() error = %v, want nil for an 85%% row count", err)
	}
}

func TestUpdater_PullAndImport_SanityCheckFailureLeavesLiveFileUntouched(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "geoip.db")
	seedRangeCounts(t, dbPath, 100)
	before, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("stat seeded db: %v", err)
	}
	beforeContent, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("read seeded db: %v", err)
	}

	// The fixture only carries 1 country range and 1 asn range - way
	// below 80% of the seeded 100.
	countryZipPath, asnZipPath := buildFixtureZips(t)
	srv, _ := fixtureServer(t, countryZipPath, asnZipPath)
	defer srv.Close()

	cfg := testConfig(dbPath)
	u := New(cfg, newTestMigrationsDir(t), nil)
	u.downloader.baseURL = srv.URL

	if err := u.pullAndImport(context.Background()); err == nil {
		t.Fatal("pullAndImport() error = nil, want a sanity-check failure")
	}

	after, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("live file vanished after a failed pull: %v", err)
	}
	if after.ModTime() != before.ModTime() || after.Size() != before.Size() {
		t.Fatal("live file's stat changed despite a sanity-check failure")
	}
	afterContent, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("read live db after failed pull: %v", err)
	}
	if string(afterContent) != string(beforeContent) {
		t.Fatal("live file's content changed despite a sanity-check failure")
	}

	entries, err := os.ReadDir(filepath.Dir(dbPath))
	if err != nil {
		t.Fatalf("read db dir: %v", err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".building-") {
			t.Errorf("leftover build artifact %q found after a failed pull", e.Name())
		}
	}
}

func TestUpdater_PullAndImport_DownloadFailureLeavesLiveFileUntouched(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "geoip.db")
	seedRangeCounts(t, dbPath, 10)
	beforeContent, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("read seeded db: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	cfg := testConfig(dbPath)
	u := New(cfg, newTestMigrationsDir(t), nil)
	u.downloader.baseURL = srv.URL

	if err := u.pullAndImport(context.Background()); err == nil {
		t.Fatal("pullAndImport() error = nil, want a download failure")
	}

	afterContent, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("read live db after failed pull: %v", err)
	}
	if string(afterContent) != string(beforeContent) {
		t.Fatal("live file's content changed despite a download failure")
	}
}

func TestUpdater_Run_StopsOnContextCancel(t *testing.T) {
	countryZipPath, asnZipPath := buildFixtureZips(t)
	srv, _ := fixtureServer(t, countryZipPath, asnZipPath)
	defer srv.Close()

	dbPath := filepath.Join(t.TempDir(), "geoip.db")
	u := New(testConfig(dbPath), newTestMigrationsDir(t), nil)
	u.downloader.baseURL = srv.URL

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		u.Run(ctx)
		close(done)
	}()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(dbPath); err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatal("Run() never performed its immediate first pull")
	}

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run() did not return after context cancellation")
	}
}
