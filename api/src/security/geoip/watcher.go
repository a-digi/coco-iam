package geoip

import (
	"context"
	"database/sql"
	"os"
	"time"

	"github.com/a-digi/coco-logger/logger"
)

// Watcher polls a geoip.db file's mtime on a fixed interval and, when
// it changes (the separate geoip-updater executable replaced it via
// atomic rename), opens a fresh connection and swaps it into the
// given SQLLookup — hot-reloading the running server's geoip data
// without a restart. This is the main-server-side half of the
// cross-process handoff described in plan/geoip-enrichment/plan.md;
// the updater never talks to this process directly, they only
// rendezvous through the file on disk.
type Watcher struct {
	path     string
	lookup   *SQLLookup
	interval time.Duration
	log      logger.Logger

	lastSeenModTime time.Time
}

// NewWatcher builds a Watcher for path, targeting lookup's connection
// slot. lookup may already be nil-connection (e.g. constructed with
// NewSQLLookup(nil) because geoip.db doesn't exist yet at boot) — the
// first tick picks it up as soon as the file appears.
func NewWatcher(path string, lookup *SQLLookup, interval time.Duration, log logger.Logger) *Watcher {
	return &Watcher{path: path, lookup: lookup, interval: interval, log: log}
}

// Run ticks on the configured interval until ctx is done, reloading
// geoip.db whenever it changes. Intended to be launched as a
// goroutine — mirrors ipguard.Sweeper.Run and dbarchive.Archiver.Run.
// Ticks once immediately before entering the loop, so a fresh boot
// against an already-populated geoip.db doesn't sit idle for up to a
// full interval before its first load — same rationale as the
// updater's own immediate first pull.
func (w *Watcher) Run(ctx context.Context) {
	w.tick()

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.tick()
		}
	}
}

// tick reloads only if path's mtime has moved forward since the last
// successful reload. Any failure (missing file, open error, a freshly
// opened connection that doesn't even respond to Ping) is logged and
// left for the next tick — the previous, still-open connection (if
// any) keeps serving lookups throughout, exactly the same
// close-only-after-a-verified-swap discipline dbarchive.rotate()
// already established for ip-attacks.db.
func (w *Watcher) tick() {
	info, err := os.Stat(w.path)
	if err != nil {
		return
	}
	if !info.ModTime().After(w.lastSeenModTime) {
		return
	}

	newDB, err := sql.Open("sqlite3", w.path)
	if err != nil {
		w.warnf("geoip: watcher failed to open %s: %v", w.path, err)
		return
	}
	if err := newDB.Ping(); err != nil {
		_ = newDB.Close()
		w.warnf("geoip: watcher failed to verify freshly-opened %s: %v", w.path, err)
		return
	}

	old := w.lookup.slot.Swap(newDB)
	w.lastSeenModTime = info.ModTime()
	if old != nil {
		_ = old.Close()
	}
	w.infof("geoip: reloaded %s (mtime %s)", w.path, w.lastSeenModTime.UTC().Format(time.RFC3339))
}

func (w *Watcher) warnf(format string, args ...interface{}) {
	if w.log != nil {
		w.log.Warning(format, args...)
	}
}

func (w *Watcher) infof(format string, args ...interface{}) {
	if w.log != nil {
		w.log.Info(format, args...)
	}
}
