// Package persistent is the write half of the port-scan-history
// repository — the scan_episodes table in ip-attacks.db (the same
// database ip_attacks/ip_attack_targets live in, a separate table).
package persistent

import (
	"fmt"
	"time"

	"github.com/a-digi/coco-iam/src/security/dbhandle"
)

const timeLayout = "2006-01-02 15:04:05"

// ScanPersistentRepo writes through a *dbhandle.Handle rather than a
// raw *sql.DB, so it keeps working across the archiver rotating
// ip-attacks.db out from under it mid-run — see
// plan/ip-attacks-db-archiving/plan.md and
// plan/port-scan-detection/plan.md's note that scanwatch should adopt
// dbhandle.Handle from day one rather than needing its own follow-up
// refactor. CreateScan is the only call that creates a new row, so
// it's the only one that touches the handle's entry counter.
type ScanPersistentRepo struct {
	handle *dbhandle.Handle
}

func NewScanPersistentRepo(handle *dbhandle.Handle) *ScanPersistentRepo {
	return &ScanPersistentRepo{handle: handle}
}

// CreateScan inserts a new scan episode row — called once, the moment
// an IP first crosses the distinct-port threshold within the
// aggregation window.
func (r *ScanPersistentRepo) CreateScan(id, ip string, startedAt time.Time) error {
	db := r.handle.DB()
	ts := startedAt.UTC().Format(timeLayout)
	_, err := db.Exec(
		`INSERT INTO scan_episodes (id, ip, started_at, last_seen_at, distinct_ports, hit_count, sample_ports)
		 VALUES (?, ?, ?, ?, 0, 0, '')`,
		id, ip, ts, ts,
	)
	if err != nil {
		return fmt.Errorf("scan-episode: create: %w", err)
	}
	if _, err := r.handle.IncrementEntryCount(db, 1); err != nil {
		return fmt.Errorf("scan-episode: create: %w", err)
	}
	return nil
}

// UpdateScan flushes the current in-memory totals for an ongoing
// episode — called periodically by scanwatch's sweeper, not per hit.
// Never creates a row, so it never touches the entry counter.
func (r *ScanPersistentRepo) UpdateScan(id string, distinctPorts, hitCount int, samplePorts string, lastSeenAt time.Time) error {
	_, err := r.handle.DB().Exec(
		`UPDATE scan_episodes SET distinct_ports = ?, hit_count = ?, sample_ports = ?, last_seen_at = ? WHERE id = ?`,
		distinctPorts, hitCount, samplePorts, lastSeenAt.UTC().Format(timeLayout), id,
	)
	if err != nil {
		return fmt.Errorf("scan-episode: update: %w", err)
	}
	return nil
}

// CloseScan marks an episode ended. Never creates a row, so it never
// touches the entry counter.
func (r *ScanPersistentRepo) CloseScan(id string, endedAt time.Time) error {
	_, err := r.handle.DB().Exec(
		`UPDATE scan_episodes SET ended_at = ? WHERE id = ?`,
		endedAt.UTC().Format(timeLayout), id,
	)
	if err != nil {
		return fmt.Errorf("scan-episode: close: %w", err)
	}
	return nil
}

// CloseAllOpen force-closes every row still open (ended_at IS NULL) by
// setting ended_at = last_seen_at, and reports how many were closed.
// Intended to be called once at startup, mirroring
// AttackPersistentRepo.CloseAllOpen's doc comment exactly: a fresh
// process's in-memory scan map always starts empty, and an episode is
// never reopened once closed, so any row still open from a prior
// process instance is definitionally orphaned.
func (r *ScanPersistentRepo) CloseAllOpen() (int64, error) {
	res, err := r.handle.DB().Exec(`UPDATE scan_episodes SET ended_at = last_seen_at WHERE ended_at IS NULL`)
	if err != nil {
		return 0, fmt.Errorf("scan-episode: close all open: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("scan-episode: close all open: rows affected: %w", err)
	}
	return n, nil
}
