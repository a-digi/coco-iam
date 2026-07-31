package ipsearch

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/a-digi/coco-iam/src/security/dbhandle"
	"github.com/a-digi/coco-sec/geoip"
)

// stubLookup is a fake geoip.Lookup keyed by exact IP string — avoids
// needing a real geoip.db (CSV import, hex-padded ranges, etc.) just
// to test Searcher's own composition/branching logic.
type stubLookup struct {
	hits map[string]geoip.Info
}

func (s *stubLookup) Lookup(ip string) (geoip.Info, bool) {
	info, ok := s.hits[ip]
	return info, ok
}

func (s *stubLookup) Enabled() bool { return true }

// freshHandle opens an in-memory ip-attacks.db-shaped SQLite database —
// mirrors api/config/db/ip_attacks_migrations/001_initial.sql,
// 002_db_meta.sql and 003_scan_episodes.sql (only the columns this
// package's queries actually touch).
func freshHandle(t *testing.T) *dbhandle.Handle {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(`
		CREATE TABLE ip_attacks (
		    id TEXT NOT NULL PRIMARY KEY,
		    ip TEXT NOT NULL
		);
		CREATE TABLE scan_episodes (
		    id TEXT NOT NULL PRIMARY KEY,
		    ip TEXT NOT NULL
		);
		CREATE TABLE db_meta (key TEXT NOT NULL PRIMARY KEY, value TEXT NOT NULL);
		INSERT INTO db_meta (key, value) VALUES ('entry_count', '0');
	`); err != nil {
		t.Fatalf("create schema: %v", err)
	}

	h, err := dbhandle.New(db)
	if err != nil {
		t.Fatalf("dbhandle.New() error = %v", err)
	}
	return h
}

func seedIPs(t *testing.T, h *dbhandle.Handle, attackIPs, scanIPs []string) {
	t.Helper()
	for i, ip := range attackIPs {
		if _, err := h.DB().Exec(`INSERT INTO ip_attacks (id, ip) VALUES (?, ?)`, "attack-"+ip+"-"+string(rune('a'+i)), ip); err != nil {
			t.Fatalf("seed ip_attacks: %v", err)
		}
	}
	for i, ip := range scanIPs {
		if _, err := h.DB().Exec(`INSERT INTO scan_episodes (id, ip) VALUES (?, ?)`, "scan-"+ip+"-"+string(rune('a'+i)), ip); err != nil {
			t.Fatalf("seed scan_episodes: %v", err)
		}
	}
}

func TestSearch_CompleteIP_Matched(t *testing.T) {
	geo := &stubLookup{hits: map[string]geoip.Info{
		"94.154.43.188": {CountryCode: "DE", Country: "Germany", ASN: 3320, ASOrg: "Deutsche Telekom AG"},
	}}
	s := NewSearcher(geo, freshHandle(t))

	results, err := s.Search("94.154.43.188", 10)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	got := results[0]
	if !got.Matched || got.IP != "94.154.43.188" || got.CountryCode != "DE" || got.ASOrg != "Deutsche Telekom AG" {
		t.Fatalf("results[0] = %+v, want a matched DE/Deutsche Telekom AG result", got)
	}
}

func TestSearch_CompleteIP_NotMatched(t *testing.T) {
	geo := &stubLookup{hits: map[string]geoip.Info{}} // no coverage at all
	s := NewSearcher(geo, freshHandle(t))

	results, err := s.Search("127.0.0.1", 10)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if results[0].Matched {
		t.Fatalf("results[0].Matched = true, want false for an address with no geoip coverage")
	}
	if results[0].IP != "127.0.0.1" {
		t.Fatalf("results[0].IP = %q, want 127.0.0.1", results[0].IP)
	}
}

func TestSearch_PartialPrefix_SuggestsKnownIPsFromBothTables(t *testing.T) {
	geo := &stubLookup{hits: map[string]geoip.Info{
		"94.154.43.188": {CountryCode: "DE"},
		"94.154.43.200": {CountryCode: "DE"},
	}}
	h := freshHandle(t)
	seedIPs(t, h,
		[]string{"94.154.43.188", "203.0.113.7"}, // ip_attacks
		[]string{"94.154.43.200"},                // scan_episodes
	)
	s := NewSearcher(geo, h)

	results, err := s.Search("94.154.43.", 10)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2 (one from each table, 203.0.113.7 excluded by prefix), got %+v", len(results), results)
	}
	for _, r := range results {
		if r.CountryCode != "DE" {
			t.Errorf("result %+v: want CountryCode DE", r)
		}
	}
}

func TestSearch_PartialPrefix_NoMatches(t *testing.T) {
	s := NewSearcher(&stubLookup{}, freshHandle(t))

	results, err := s.Search("10.0.", 10)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("len(results) = %d, want 0", len(results))
	}
}

func TestSearch_EmptyQuery_Errors(t *testing.T) {
	s := NewSearcher(&stubLookup{}, freshHandle(t))

	if _, err := s.Search("   ", 10); err == nil {
		t.Fatal("Search(\"   \") error = nil, want an error for an effectively-empty query")
	}
}

func TestSearch_LimitClamping(t *testing.T) {
	h := freshHandle(t)
	ips := make([]string, 0, 30)
	for i := 0; i < 30; i++ {
		ips = append(ips, "203.0.113."+string(rune('0'+i%10))+string(rune('0'+i/10)))
	}
	seedIPs(t, h, ips, nil)
	s := NewSearcher(&stubLookup{}, h)

	cases := []struct {
		name      string
		requested int
		want      int
	}{
		{"zero defaults", 0, DefaultLimit},
		{"negative defaults", -5, DefaultLimit},
		{"over max clamps", 1000, MaxLimit},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			results, err := s.Search("203.0.113.", tc.requested)
			if err != nil {
				t.Fatalf("Search() error = %v", err)
			}
			if len(results) != tc.want {
				t.Fatalf("len(results) = %d, want %d", len(results), tc.want)
			}
		})
	}
}

func TestSearch_LikeWildcardsInQueryAreEscaped(t *testing.T) {
	h := freshHandle(t)
	seedIPs(t, h, []string{"203.0.113.7"}, nil)
	s := NewSearcher(&stubLookup{}, h)

	// A literal "_" must not act as a SQL LIKE single-character
	// wildcard — "1_3" should not incidentally match "113" style
	// addresses via wildcard expansion.
	results, err := s.Search("203.0.113_7", 10)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("len(results) = %d, want 0 (literal underscore must not wildcard-match)", len(results))
	}
}
