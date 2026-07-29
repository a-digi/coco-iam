package geoip

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// buildGeoIPFile creates a real on-disk SQLite file at path, migrated
// via the real schema (see migration001 in schema_test.go), with one
// country range seeded for country. Returns the *sql.DB used to build
// it — callers must Close it before renaming the file elsewhere
// (Watcher.tick opens its own connection against the path).
func buildGeoIPFile(t *testing.T, path, country string) {
	t.Helper()
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	if _, err := db.Exec(migration001); err != nil {
		t.Fatalf("apply schema to %s: %v", path, err)
	}
	_, startHex, _ := encodeIP("1.2.3.0")
	_, endHex, _ := encodeIP("1.2.3.255")
	seedCountryRange(t, db, "v4", startHex, endHex, "XX", country)
	if err := db.Close(); err != nil {
		t.Fatalf("close %s: %v", path, err)
	}
}

func TestWatcher_LoadsAlreadyExistingFileOnFirstTick(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "geoip.db")
	buildGeoIPFile(t, path, "Wonderland")

	lookup := NewSQLLookup(nil)
	w := NewWatcher(path, lookup, time.Hour, nil)
	w.tick()

	info, ok := lookup.Lookup("1.2.3.42")
	if !ok || info.Country != "Wonderland" {
		t.Fatalf("Lookup() = %+v, ok=%v, want Wonderland/true", info, ok)
	}
}

func TestWatcher_TickIsNoopWhenFileUnchanged(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "geoip.db")
	buildGeoIPFile(t, path, "Wonderland")

	lookup := NewSQLLookup(nil)
	w := NewWatcher(path, lookup, time.Hour, nil)
	w.tick()

	before := lookup.slot.DB()
	w.tick() // second tick, file untouched
	after := lookup.slot.DB()

	if before != after {
		t.Fatal("tick() swapped in a new connection even though the file's mtime never changed")
	}
}

func TestWatcher_PicksUpReplacedFileOnNextTick(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "geoip.db")
	buildGeoIPFile(t, path, "Wonderland")

	lookup := NewSQLLookup(nil)
	w := NewWatcher(path, lookup, time.Hour, nil)
	w.tick()

	if info, ok := lookup.Lookup("1.2.3.42"); !ok || info.Country != "Wonderland" {
		t.Fatalf("initial Lookup() = %+v, ok=%v, want Wonderland/true", info, ok)
	}

	// Simulate the updater: build a brand-new file elsewhere, then
	// atomically rename it over the live path — exactly what
	// pullAndImport will do in production.
	replacement := filepath.Join(dir, "geoip.db.building")
	buildGeoIPFile(t, replacement, "Neverland")
	// Ensure the replacement's mtime is observably later than the
	// original's — same-second renames on a coarse filesystem clock
	// could otherwise make this test flaky.
	future := time.Now().Add(time.Second)
	if err := os.Chtimes(replacement, future, future); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	if err := os.Rename(replacement, path); err != nil {
		t.Fatalf("rename: %v", err)
	}

	w.tick()

	info, ok := lookup.Lookup("1.2.3.42")
	if !ok || info.Country != "Neverland" {
		t.Fatalf("Lookup() after reload = %+v, ok=%v, want Neverland/true", info, ok)
	}
}

func TestWatcher_KeepsServingOldConnectionWhenFileGoesMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "geoip.db")
	buildGeoIPFile(t, path, "Wonderland")

	lookup := NewSQLLookup(nil)
	w := NewWatcher(path, lookup, time.Hour, nil)
	w.tick()

	if err := os.Remove(path); err != nil {
		t.Fatalf("remove: %v", err)
	}
	w.tick() // must not panic, must not clear the existing connection

	info, ok := lookup.Lookup("1.2.3.42")
	if !ok || info.Country != "Wonderland" {
		t.Fatalf("Lookup() after the file vanished = %+v, ok=%v, want the old data to still be served", info, ok)
	}
}

func TestWatcher_NeverLoadsWhenFileNeverExisted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "never-created.db")

	lookup := NewSQLLookup(nil)
	w := NewWatcher(path, lookup, time.Hour, nil)
	w.tick()

	if _, ok := lookup.Lookup("1.2.3.42"); ok {
		t.Fatal("Lookup() ok = true, want false when geoip.db was never created")
	}
}

func TestWatcher_RunStopsOnContextCancel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "geoip.db")
	buildGeoIPFile(t, path, "Wonderland")

	lookup := NewSQLLookup(nil)
	w := NewWatcher(path, lookup, time.Millisecond, nil)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		w.Run(ctx)
		close(done)
	}()

	// Run's immediate first tick should have loaded the file already.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, ok := lookup.Lookup("1.2.3.42"); ok {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if _, ok := lookup.Lookup("1.2.3.42"); !ok {
		t.Fatal("Run() never loaded geoip.db via its immediate first tick")
	}

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run() did not return after context cancellation")
	}
}
