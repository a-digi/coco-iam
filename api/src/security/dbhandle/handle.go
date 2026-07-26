// Package dbhandle provides a swappable *sql.DB wrapper for
// ip-attacks.db, so ipguard and scanwatch never hold a raw connection
// that goes stale the moment the archiver rotates the file out from
// under them. See plan/ip-attacks-db-archiving/plan.md.
//
// Every persistence call must go through DB() rather than storing a
// *sql.DB directly — that's what makes rotation possible without
// reconstructing either consumer.
package dbhandle

import (
	"database/sql"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
)

const entryCountKey = "entry_count"

// Handle wraps the current ip-attacks.db connection alongside a
// crash-safe running count of rows inserted since the last rotation.
// The count is checked against the archive threshold without a full
// COUNT(*) scan across tables that may hold tens of millions of rows.
type Handle struct {
	mu sync.RWMutex
	db *sql.DB

	entryCount atomic.Int64
}

// New wraps db, reading the persisted entry_count from db_meta so a
// restart resumes counting from where the last run left off instead
// of from zero.
func New(db *sql.DB) (*Handle, error) {
	if db == nil {
		return nil, fmt.Errorf("dbhandle: db must not be nil")
	}

	count, err := readEntryCount(db)
	if err != nil {
		return nil, fmt.Errorf("dbhandle: failed to load entry_count: %w", err)
	}

	h := &Handle{db: db}
	h.entryCount.Store(count)
	return h, nil
}

// DB returns the current connection. Safe to call concurrently with
// Swap — a caller either gets the old connection or the new one, never
// a half-swapped state.
func (h *Handle) DB() *sql.DB {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.db
}

// Swap replaces the underlying connection. Called only by the
// archiver, once it has fully prepared and migrated a fresh
// generation — callers blocked in DB() unblock immediately after with
// the new connection.
func (h *Handle) Swap(newDB *sql.DB) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.db = newDB
}

// EntryCount returns the current in-memory running total.
func (h *Handle) EntryCount() int64 {
	return h.entryCount.Load()
}

// IncrementEntryCount adds n to the running total and persists the
// updated value to db_meta on db — the same connection the caller just
// used to write the row(s) being counted, so the two never disagree
// even if a rotation lands between one persistence call and the next.
// Returns the updated total so the archiver can compare it against the
// rotation threshold without a separate read.
func (h *Handle) IncrementEntryCount(db *sql.DB, n int64) (int64, error) {
	updated := h.entryCount.Add(n)

	if _, err := db.Exec(`UPDATE db_meta SET value = ? WHERE key = ?`, strconv.FormatInt(updated, 10), entryCountKey); err != nil {
		return updated, fmt.Errorf("dbhandle: failed to persist entry_count: %w", err)
	}

	return updated, nil
}

// ResetEntryCount zeroes the counter, in memory and in db_meta. Called
// by the archiver immediately after Swap, so the fresh generation
// starts counting from 0.
func (h *Handle) ResetEntryCount(db *sql.DB) error {
	h.entryCount.Store(0)

	if _, err := db.Exec(`UPDATE db_meta SET value = ? WHERE key = ?`, "0", entryCountKey); err != nil {
		return fmt.Errorf("dbhandle: failed to reset entry_count: %w", err)
	}

	return nil
}

func readEntryCount(db *sql.DB) (int64, error) {
	var raw string
	err := db.QueryRow(`SELECT value FROM db_meta WHERE key = ?`, entryCountKey).Scan(&raw)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}

	count, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid entry_count value %q: %w", raw, err)
	}

	return count, nil
}
