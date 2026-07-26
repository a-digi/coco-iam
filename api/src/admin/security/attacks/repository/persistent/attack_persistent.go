// Package persistent is the write half of the attack-history
// repository — ip-attacks.db (a separate database from the main one),
// ip_attacks + ip_attack_targets tables.
package persistent

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const timeLayout = "2006-01-02 15:04:05"

type AttackPersistentRepo struct {
	db *sql.DB
}

func NewAttackPersistentRepo(db *sql.DB) *AttackPersistentRepo {
	return &AttackPersistentRepo{db: db}
}

// CreateAttack inserts a new attack episode row with the given id —
// called once, the moment an IP's first rejected request has no
// already-open episode.
func (r *AttackPersistentRepo) CreateAttack(id, ip, tier string, startedAt time.Time) error {
	ts := startedAt.UTC().Format(timeLayout)
	_, err := r.db.Exec(
		`INSERT INTO ip_attacks (id, ip, tier, started_at, last_seen_at, hit_count, ban_count)
		 VALUES (?, ?, ?, ?, ?, 0, 1)`,
		id, ip, tier, ts, ts,
	)
	if err != nil {
		return fmt.Errorf("ip-attack: create: %w", err)
	}
	return nil
}

// UpdateAttackCounts flushes the current in-memory totals for an
// ongoing episode — called periodically by the sweeper, not per hit.
func (r *AttackPersistentRepo) UpdateAttackCounts(id string, hitCount, banCount int, lastSeenAt time.Time) error {
	_, err := r.db.Exec(
		`UPDATE ip_attacks SET hit_count = ?, ban_count = ?, last_seen_at = ? WHERE id = ?`,
		hitCount, banCount, lastSeenAt.UTC().Format(timeLayout), id,
	)
	if err != nil {
		return fmt.Errorf("ip-attack: update counts: %w", err)
	}
	return nil
}

// CloseAttack marks an episode ended.
func (r *AttackPersistentRepo) CloseAttack(id string, endedAt time.Time) error {
	_, err := r.db.Exec(
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
	res, err := r.db.Exec(`UPDATE ip_attacks SET ended_at = last_seen_at WHERE ended_at IS NULL`)
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
func (r *AttackPersistentRepo) SetTargetCount(attackID, path, method string, hitCount int) error {
	_, err := r.db.Exec(
		`INSERT INTO ip_attack_targets (id, attack_id, path, method, hit_count)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(attack_id, path, method) DO UPDATE SET hit_count = excluded.hit_count`,
		uuid.New().String(), attackID, path, method, hitCount,
	)
	if err != nil {
		return fmt.Errorf("ip-attack: set target count: %w", err)
	}
	return nil
}
