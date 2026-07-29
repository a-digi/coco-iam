// Package persistent is the write half of the attack-history
// repository — ip-attacks.db (a separate database from the main one),
// ip_attacks + ip_attack_targets tables.
package persistent

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/a-digi/coco-iam/src/security/dbhandle"
)

const timeLayout = "2006-01-02 15:04:05"

// AttackPersistentRepo writes through a *dbhandle.Handle rather than a
// raw *sql.DB, so it keeps working across the archiver rotating
// ip-attacks.db out from under it mid-run — see
// plan/ip-attacks-db-archiving/plan.md. Every call that creates a new
// row (CreateAttack, and SetTargetCount when it's actually an insert)
// also increments the handle's entry counter on the same connection it
// just wrote to, so the two can never disagree.
type AttackPersistentRepo struct {
	handle *dbhandle.Handle
}

func NewAttackPersistentRepo(handle *dbhandle.Handle) *AttackPersistentRepo {
	return &AttackPersistentRepo{handle: handle}
}

// CreateAttack inserts a new attack episode row with the given id —
// called once, the moment an IP's first rejected request has no
// already-open episode. originHint is a JSON snapshot of whatever
// client-identifying headers were present on that first request,
// captured only when ip resolved to a loopback/private address (i.e.
// the configured trust header(s) found nothing usable) — nil in the
// normal case where ip was resolved successfully. See
// plan/attack-ip-attribution/plan.md Fix 3.
func (r *AttackPersistentRepo) CreateAttack(id, ip, tier string, startedAt time.Time, originHint *string) error {
	db := r.handle.DB()
	ts := startedAt.UTC().Format(timeLayout)
	var originHintArg interface{}
	if originHint != nil {
		originHintArg = *originHint
	}
	_, err := db.Exec(
		`INSERT INTO ip_attacks (id, ip, tier, started_at, last_seen_at, hit_count, ban_count, origin_hint)
		 VALUES (?, ?, ?, ?, ?, 0, 1, ?)`,
		id, ip, tier, ts, ts, originHintArg,
	)
	if err != nil {
		return fmt.Errorf("ip-attack: create: %w", err)
	}
	if _, err := r.handle.IncrementEntryCount(db, 1); err != nil {
		return fmt.Errorf("ip-attack: create: %w", err)
	}
	return nil
}

// UpdateAttackCounts flushes the current in-memory totals for an
// ongoing episode — called periodically by the sweeper, not per hit.
// Never creates a row, so it never touches the entry counter.
func (r *AttackPersistentRepo) UpdateAttackCounts(id string, hitCount, banCount int, lastSeenAt time.Time) error {
	_, err := r.handle.DB().Exec(
		`UPDATE ip_attacks SET hit_count = ?, ban_count = ?, last_seen_at = ? WHERE id = ?`,
		hitCount, banCount, lastSeenAt.UTC().Format(timeLayout), id,
	)
	if err != nil {
		return fmt.Errorf("ip-attack: update counts: %w", err)
	}
	return nil
}

// CloseAttack marks an episode ended. Never creates a row, so it never
// touches the entry counter.
func (r *AttackPersistentRepo) CloseAttack(id string, endedAt time.Time) error {
	_, err := r.handle.DB().Exec(
		`UPDATE ip_attacks SET ended_at = ? WHERE id = ?`,
		endedAt.UTC().Format(timeLayout), id,
	)
	if err != nil {
		return fmt.Errorf("ip-attack: close: %w", err)
	}
	return nil
}

// CloseAllOpen force-closes every row still open (ended_at IS NULL)
// by setting ended_at = last_seen_at, and reports how many were
// closed. Intended to be called once at startup: a fresh process's
// in-memory attack map always starts empty, and an episode is never
// reopened once closed (a later attack from the same IP is a new
// row — see plan/ip-abuse-protection/plan.md section 11), so any row
// still open from a prior process instance is definitionally
// orphaned and should be reconciled immediately rather than left
// open forever.
func (r *AttackPersistentRepo) CloseAllOpen() (int64, error) {
	res, err := r.handle.DB().Exec(`UPDATE ip_attacks SET ended_at = last_seen_at WHERE ended_at IS NULL`)
	if err != nil {
		return 0, fmt.Errorf("ip-attack: close all open: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("ip-attack: close all open: rows affected: %w", err)
	}
	return n, nil
}

// SetTargetCount sets the absolute hit_count for (attackID, path,
// method), creating the row if absent. Takes an absolute value, not a
// delta — the caller always knows and passes the current in-memory
// total, so there's nothing to reconcile if a flush is ever retried.
// The ON CONFLICT upsert doesn't tell us whether it inserted or
// updated, so a target that's already been flushed once (the common
// case — targets are flushed on every sweeper tick) has to be checked
// for first; the entry counter must only move on a genuine new row.
//
// bodySample is the first-observed (redacted, size-capped) request
// body for this target — nil if none was captured. Only ever written
// on the INSERT branch: the ON CONFLICT clause below deliberately
// omits body_sample from its SET list, so a target that already has a
// stored sample keeps it across every later flush instead of being
// overwritten by whatever the most recent hit happened to send. See
// plan/attack-ip-attribution/plan.md Fix 2.
func (r *AttackPersistentRepo) SetTargetCount(attackID, path, method string, hitCount int, bodySample *string) error {
	db := r.handle.DB()

	var exists int
	err := db.QueryRow(
		`SELECT 1 FROM ip_attack_targets WHERE attack_id = ? AND path = ? AND method = ?`,
		attackID, path, method,
	).Scan(&exists)
	isNew := err == sql.ErrNoRows
	if err != nil && !isNew {
		return fmt.Errorf("ip-attack: set target count: check existing: %w", err)
	}

	var bodySampleArg interface{}
	if bodySample != nil {
		bodySampleArg = *bodySample
	}
	_, err = db.Exec(
		`INSERT INTO ip_attack_targets (id, attack_id, path, method, hit_count, body_sample)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(attack_id, path, method) DO UPDATE SET hit_count = excluded.hit_count`,
		uuid.New().String(), attackID, path, method, hitCount, bodySampleArg,
	)
	if err != nil {
		return fmt.Errorf("ip-attack: set target count: %w", err)
	}

	if isNew {
		if _, err := r.handle.IncrementEntryCount(db, 1); err != nil {
			return fmt.Errorf("ip-attack: set target count: %w", err)
		}
	}

	return nil
}
