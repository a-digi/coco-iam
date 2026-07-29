package geoip

import (
	"database/sql"
	"sync"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func openMemDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestDBSlot_DBReturnsInitialConnection(t *testing.T) {
	db := openMemDB(t)
	slot := newDBSlot(db)

	if got := slot.DB(); got != db {
		t.Fatalf("DB() = %v, want the connection passed to newDBSlot", got)
	}
}

func TestDBSlot_DBReturnsNilWhenConstructedEmpty(t *testing.T) {
	slot := newDBSlot(nil)
	if got := slot.DB(); got != nil {
		t.Fatalf("DB() = %v, want nil", got)
	}
}

func TestDBSlot_SwapReplacesConnectionAndReturnsOld(t *testing.T) {
	oldDB := openMemDB(t)
	newDB := openMemDB(t)
	slot := newDBSlot(oldDB)

	returned := slot.Swap(newDB)
	if returned != oldDB {
		t.Fatalf("Swap() returned = %v, want the old connection", returned)
	}
	if got := slot.DB(); got != newDB {
		t.Fatalf("DB() after Swap = %v, want the new connection", got)
	}
}

func TestDBSlot_ConcurrentDBAndSwapAreRaceFree(t *testing.T) {
	slot := newDBSlot(openMemDB(t))

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = slot.DB()
		}()
		go func() {
			defer wg.Done()
			// sql.Open only parses the DSN and never errors for the
			// sqlite3 driver — safe to ignore here without a t.Fatalf,
			// which testing forbids calling from a non-test goroutine.
			replacement, _ := sql.Open("sqlite3", ":memory:")
			old := slot.Swap(replacement)
			if old != nil {
				_ = old.Close()
			}
		}()
	}
	wg.Wait()
}
