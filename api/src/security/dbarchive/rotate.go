package dbarchive

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	dbmanager "github.com/a-digi/coco-orm/orm"
)

// rotate performs the actual file swap. Every step before closing the
// live connection is purely preparatory (stat, mkdir) and aborts
// cleanly with the live db untouched on failure. Once the connection
// is closed there is no way to make the remaining steps atomic across
// two separate SQLite files plus a filesystem rename, so failures past
// that point are handled with best-effort recovery (reopening the
// original file, or rolling the rename back) rather than a true
// rollback — see plan/ip-attacks-db-archiving/plan.md's "Rotation
// safety" note.
func (a *Archiver) rotate() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Re-check under the lock: another call may have already rotated
	// between CheckAndRotate's read and here.
	if a.handle.EntryCount() < a.threshold {
		return nil
	}

	livePath := filepath.Join(a.dbDir, a.dbName)
	rowCount := a.handle.EntryCount()

	info, err := os.Stat(livePath)
	if err != nil {
		return fmt.Errorf("dbarchive: stat live db: %w", err)
	}
	sizeBytes := info.Size()
	startedAt := earliestStartedAt(a.handle.DB())

	if err := os.MkdirAll(a.archiveDir, 0755); err != nil {
		return fmt.Errorf("dbarchive: create archive directory: %w", err)
	}

	archivedAt := time.Now().UTC()
	base := strings.TrimSuffix(a.dbName, filepath.Ext(a.dbName))
	archivePath := filepath.Join(a.archiveDir, fmt.Sprintf("%s-%d.db", base, archivedAt.Unix()))

	if err := a.manager.Connector.Close(); err != nil {
		return fmt.Errorf("dbarchive: close live connection: %w", err)
	}

	if err := os.Rename(livePath, archivePath); err != nil {
		a.reopenAt() // nothing moved — the original file is still there
		return fmt.Errorf("dbarchive: move live db to archive: %w", err)
	}

	if err := a.completeRotation(archivePath, startedAt, archivedAt, rowCount, sizeBytes); err != nil {
		// The old generation is sitting at archivePath but nothing
		// registers it as such yet — put it back exactly where it was
		// rather than leave a valid generation half-migrated into an
		// untracked archive.
		_ = os.Remove(livePath) // best-effort: clears any partial fresh file completeRotation left behind
		_ = os.Rename(archivePath, livePath)
		a.reopenAt()
		return err
	}

	return nil
}

// completeRotation builds the fresh generation and registers the
// rotated-out one. Only called once the old generation is already
// sitting at archivePath. It never touches a.manager/a.handle on
// failure — the caller owns rolling the rename back and reopening.
func (a *Archiver) completeRotation(archivePath, startedAt string, archivedAt time.Time, rowCount, sizeBytes int64) error {
	freshManager, err := dbmanager.NewDatabaseManager(a.dbName, a.dbDir, []string{a.migrationsPath})
	if err != nil {
		return fmt.Errorf("dbarchive: create fresh generation: %w", err)
	}
	if err := freshManager.SyncMigrations(); err != nil {
		_ = freshManager.Connector.Close()
		return fmt.Errorf("dbarchive: migrate fresh generation: %w", err)
	}
	if err := a.insertArchiveRow(archivePath, startedAt, archivedAt, rowCount, sizeBytes); err != nil {
		_ = freshManager.Connector.Close()
		return fmt.Errorf("dbarchive: register archive: %w", err)
	}

	a.manager = freshManager
	a.handle.Swap(freshManager.Connector.DB)
	if err := a.handle.ResetEntryCount(freshManager.Connector.DB); err != nil {
		return fmt.Errorf("dbarchive: reset entry count on fresh generation: %w", err)
	}
	return nil
}

// reopenAt best-effort reopens the manager at the canonical live path
// after a failed rotation, so the caller (the periodic ticker) doesn't
// leave the process without any ip-attacks.db connection at all just
// because one rotation attempt failed. If even this fails, a.manager
// and a.handle keep pointing at the already-closed connection — the
// caller's returned error surfaces the failure either way, and the
// next tick will try again.
func (a *Archiver) reopenAt() {
	reopened, err := dbmanager.NewDatabaseManager(a.dbName, a.dbDir, []string{a.migrationsPath})
	if err != nil {
		return
	}
	a.manager = reopened
	a.handle.Swap(reopened.Connector.DB)
}

// earliestStartedAt returns the earliest ip_attacks.started_at in db,
// formatted for storage in ip_attacks_archives — or "now" if this
// generation never recorded an attack (nothing to derive a start time
// from). The driver recognizes MIN()'s result as the column's
// DATETIME type and hands back RFC3339 (unlike, say, wrapping a column
// in COALESCE, which loses that type metadata and comes back as a
// plain string) — parsed defensively against either shape anyway,
// the same way ipguard's own parseTime does for ip_bans.expires_at.
func earliestStartedAt(db *sql.DB) string {
	var raw sql.NullString
	if err := db.QueryRow(`SELECT MIN(started_at) FROM ip_attacks`).Scan(&raw); err != nil || !raw.Valid {
		return time.Now().UTC().Format(timeLayout)
	}
	for _, layout := range []string{timeLayout, time.RFC3339, time.RFC3339Nano} {
		if t, err := time.Parse(layout, raw.String); err == nil {
			return t.UTC().Format(timeLayout)
		}
	}
	return time.Now().UTC().Format(timeLayout)
}

func (a *Archiver) insertArchiveRow(filePath, startedAt string, archivedAt time.Time, rowCount, sizeBytes int64) error {
	_, err := a.mainDB.Exec(
		`INSERT INTO ip_attacks_archives (id, file_path, started_at, archived_at, row_count, size_bytes)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		uuid.New().String(), filePath, startedAt, archivedAt.Format(timeLayout), rowCount, sizeBytes,
	)
	if err != nil {
		return fmt.Errorf("insert ip_attacks_archives row: %w", err)
	}
	return nil
}
