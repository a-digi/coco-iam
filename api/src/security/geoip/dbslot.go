package geoip

import (
	"database/sql"
	"sync"
)

// dbSlot holds the currently-active *sql.DB connection to geoip.db,
// swappable without reconstructing any consumer. Mirrors
// dbhandle.Handle's DB()/Swap() mechanism (api/src/security/dbhandle)
// deliberately without its entry-count tracking, which is specific to
// ip-attacks.db's archive-rotation threshold and doesn't apply here —
// geoip.db is always fully replaced wholesale by the separate
// geoip-updater process, never grown in place until a threshold. See
// plan/geoip-enrichment/plan.md.
type dbSlot struct {
	mu sync.RWMutex
	db *sql.DB
}

// newDBSlot wraps db as the initial connection. db may be nil — a
// slot with no connection yet is valid (e.g. before Watcher's first
// successful open), DB() just returns nil and callers must treat that
// as "no data available".
func newDBSlot(db *sql.DB) *dbSlot {
	return &dbSlot{db: db}
}

// DB returns the current connection. Safe to call concurrently with
// Swap — a caller either gets the old connection or the new one,
// never a half-swapped state.
func (s *dbSlot) DB() *sql.DB {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.db
}

// Swap replaces the current connection and returns the previous one
// (nil if there wasn't one yet) so the caller can close it immediately
// after Swap returns — the same "close right after swap" discipline
// already established by dbarchive.rotate() for ip-attacks.db.
func (s *dbSlot) Swap(newDB *sql.DB) *sql.DB {
	s.mu.Lock()
	defer s.mu.Unlock()
	old := s.db
	s.db = newDB
	return old
}
