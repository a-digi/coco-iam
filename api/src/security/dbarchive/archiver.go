// Package dbarchive rotates ip-attacks.db out once its running entry
// count crosses a configured threshold, registering the rotated-out
// file in the main DB's ip_attacks_archives table so it stays
// queryable later instead of becoming an opaque, forgotten file on
// disk. See plan/ip-attacks-db-archiving/plan.md.
package dbarchive

import (
	"context"
	"database/sql"
	"sync"
	"time"

	"github.com/a-digi/coco-logger/logger"
	dbmanager "github.com/a-digi/coco-orm/orm"

	"github.com/a-digi/coco-iam/src/security/dbhandle"
)

// sweepInterval is how often Run polls CheckAndRotate — cheap when
// below threshold (a single in-memory read), so a short interval costs
// nothing; kept separate from ipguard.Sweeper's 5-minute tick since
// rotation is a distinct, much rarer concern.
const sweepInterval = 10 * time.Minute

// timeLayout matches every other ip-attacks.db/ip_attacks_archives
// timestamp in this codebase — see ipguard's and
// AttackPersistentRepo's own timeLayout constants.
const timeLayout = "2006-01-02 15:04:05"

// DefaultThreshold is the entry count (summed across every table in
// ip-attacks.db, via the single db_meta counter) at which a rotation
// fires — the "100 million entries" requirement in
// plan/ip-attacks-db-archiving/plan.md.
const DefaultThreshold = 100_000_000

// Archiver rotates ip-attacks.db once handle's entry counter crosses
// threshold. Nothing here ever deletes a rotated-out file — it's
// moved into archiveDir and registered in the main DB's
// ip_attacks_archives table so it stays queryable (see Open Question 1
// in the plan for what "reused" was taken to mean).
type Archiver struct {
	mu sync.Mutex

	handle  *dbhandle.Handle
	manager *dbmanager.DatabaseManager // the live ip-attacks.db manager; replaced on every successful rotation
	mainDB  *sql.DB                    // holds ip_attacks_archives — the main DB, never itself rotated

	dbName         string
	dbDir          string
	migrationsPath string
	archiveDir     string
	threshold      int64

	log logger.Logger
}

// New builds an Archiver. manager must be the same DatabaseManager
// currently backing handle (i.e. manager.Connector.DB == handle.DB()) —
// the two are kept in lockstep across every rotation. log may be nil.
func New(handle *dbhandle.Handle, manager *dbmanager.DatabaseManager, mainDB *sql.DB, dbName, dbDir, migrationsPath, archiveDir string, threshold int64, log logger.Logger) *Archiver {
	return &Archiver{
		handle:         handle,
		manager:        manager,
		mainDB:         mainDB,
		dbName:         dbName,
		dbDir:          dbDir,
		migrationsPath: migrationsPath,
		archiveDir:     archiveDir,
		threshold:      threshold,
		log:            log,
	}
}

// Run blocks, polling CheckAndRotate every sweepInterval, until ctx is
// cancelled. Intended to be launched as a goroutine — mirrors
// oauthserver/archiver.Archiver.Run's shape.
func (a *Archiver) Run(ctx context.Context) {
	ticker := time.NewTicker(sweepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := a.CheckAndRotate(); err != nil && a.log != nil {
				a.log.Warning("dbarchive: rotation check failed: %v", err)
			}
		}
	}
}

// CheckAndRotate rotates ip-attacks.db if its entry count has crossed
// the threshold — a cheap in-memory read on every call that doesn't,
// which is the overwhelming majority of calls. Intended to be polled
// on a fixed interval by its own ticker (see plan section "Rotation
// tick cadence") separate from ipguard's own Sweeper.
func (a *Archiver) CheckAndRotate() error {
	if a.handle.EntryCount() < a.threshold {
		return nil
	}
	return a.rotate()
}

// Manager returns the DatabaseManager currently backing the live
// ip-attacks.db — for callers (e.g. a future ContextBag wiring step)
// that need to keep a read path in sync with rotations too.
func (a *Archiver) Manager() *dbmanager.DatabaseManager {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.manager
}

// EntryCount returns the live generation's current running entry
// count — for the admin security status endpoint, so the Security
// page can show how close the next rotation is.
func (a *Archiver) EntryCount() int64 {
	return a.handle.EntryCount()
}

// Threshold returns the entry count at which the next rotation fires.
func (a *Archiver) Threshold() int64 {
	return a.threshold
}
