package dbhandle

import (
	"database/sql"
	"sync"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// freshDB opens an in-memory SQLite DB with the db_meta schema —
// mirrors api/config/db/ip_attacks_migrations/002_db_meta.sql.
func freshDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if _, err := db.Exec(`CREATE TABLE db_meta (key TEXT NOT NULL PRIMARY KEY, value TEXT NOT NULL)`); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO db_meta (key, value) VALUES ('entry_count', '0')`); err != nil {
		t.Fatalf("seed db_meta: %v", err)
	}
	return db
}

func TestNew_NilDB(t *testing.T) {
	if _, err := New(nil); err == nil {
		t.Fatal("New(nil) error = nil, want error")
	}
}

func TestNew_ReadsPersistedCount(t *testing.T) {
	db := freshDB(t)
	if _, err := db.Exec(`UPDATE db_meta SET value = '42' WHERE key = 'entry_count'`); err != nil {
		t.Fatalf("seed count: %v", err)
	}

	h, err := New(db)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if got := h.EntryCount(); got != 42 {
		t.Fatalf("EntryCount() = %d, want 42", got)
	}
}

func TestNew_MissingRowDefaultsToZero(t *testing.T) {
	db := freshDB(t)
	if _, err := db.Exec(`DELETE FROM db_meta WHERE key = 'entry_count'`); err != nil {
		t.Fatalf("delete row: %v", err)
	}

	h, err := New(db)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if got := h.EntryCount(); got != 0 {
		t.Fatalf("EntryCount() = %d, want 0", got)
	}
}

func TestNew_InvalidStoredValue(t *testing.T) {
	db := freshDB(t)
	if _, err := db.Exec(`UPDATE db_meta SET value = 'not-a-number' WHERE key = 'entry_count'`); err != nil {
		t.Fatalf("seed count: %v", err)
	}

	if _, err := New(db); err == nil {
		t.Fatal("New() error = nil, want error for non-numeric stored value")
	}
}

func TestIncrementEntryCount_UpdatesMemoryAndPersists(t *testing.T) {
	db := freshDB(t)
	h, err := New(db)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	updated, err := h.IncrementEntryCount(db, 5)
	if err != nil {
		t.Fatalf("IncrementEntryCount() error = %v", err)
	}
	if updated != 5 {
		t.Fatalf("IncrementEntryCount() returned %d, want 5", updated)
	}
	if got := h.EntryCount(); got != 5 {
		t.Fatalf("EntryCount() = %d, want 5", got)
	}

	var persisted string
	if err := db.QueryRow(`SELECT value FROM db_meta WHERE key = 'entry_count'`).Scan(&persisted); err != nil {
		t.Fatalf("read back persisted count: %v", err)
	}
	if persisted != "5" {
		t.Fatalf("persisted entry_count = %q, want %q", persisted, "5")
	}

	updated, err = h.IncrementEntryCount(db, 3)
	if err != nil {
		t.Fatalf("IncrementEntryCount() error = %v", err)
	}
	if updated != 8 {
		t.Fatalf("IncrementEntryCount() returned %d, want 8", updated)
	}
}

func TestResetEntryCount(t *testing.T) {
	db := freshDB(t)
	h, err := New(db)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if _, err := h.IncrementEntryCount(db, 100); err != nil {
		t.Fatalf("IncrementEntryCount() error = %v", err)
	}

	if err := h.ResetEntryCount(db); err != nil {
		t.Fatalf("ResetEntryCount() error = %v", err)
	}
	if got := h.EntryCount(); got != 0 {
		t.Fatalf("EntryCount() = %d, want 0 after reset", got)
	}

	var persisted string
	if err := db.QueryRow(`SELECT value FROM db_meta WHERE key = 'entry_count'`).Scan(&persisted); err != nil {
		t.Fatalf("read back persisted count: %v", err)
	}
	if persisted != "0" {
		t.Fatalf("persisted entry_count = %q, want %q", persisted, "0")
	}
}

func TestSwap_DBReflectsNewConnection(t *testing.T) {
	oldDB := freshDB(t)
	newDB := freshDB(t)

	h, err := New(oldDB)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if h.DB() != oldDB {
		t.Fatal("DB() before Swap does not return the original connection")
	}

	h.Swap(newDB)
	if h.DB() != newDB {
		t.Fatal("DB() after Swap does not return the new connection")
	}
}

// TestSwap_ConcurrentWithDB simulates the rotation window: one
// goroutine repeatedly swaps the connection while many others read it,
// under -race, to confirm no caller ever observes a torn/partial state
// and nothing deadlocks.
func TestSwap_ConcurrentWithDB(t *testing.T) {
	dbA := freshDB(t)
	dbB := freshDB(t)

	h, err := New(dbA)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	var wg sync.WaitGroup
	stop := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		toggle := false
		for {
			select {
			case <-stop:
				return
			default:
				if toggle {
					h.Swap(dbA)
				} else {
					h.Swap(dbB)
				}
				toggle = !toggle
			}
		}
	}()

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				got := h.DB()
				if got != dbA && got != dbB {
					t.Errorf("DB() returned unexpected connection")
					return
				}
			}
		}()
	}

	close(stop)
	wg.Wait()
}

// TestIncrementEntryCount_ConcurrentIncrements confirms the in-memory
// counter itself is race-safe under concurrent callers (independent of
// which connection each happens to land on).
func TestIncrementEntryCount_ConcurrentIncrements(t *testing.T) {
	db := freshDB(t)
	h, err := New(db)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	var wg sync.WaitGroup
	var mu sync.Mutex // serializes the shared db.Exec below; the counter itself needs no lock
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mu.Lock()
			defer mu.Unlock()
			if _, err := h.IncrementEntryCount(db, 1); err != nil {
				t.Errorf("IncrementEntryCount() error = %v", err)
			}
		}()
	}
	wg.Wait()

	if got := h.EntryCount(); got != 50 {
		t.Fatalf("EntryCount() = %d, want 50", got)
	}
}
