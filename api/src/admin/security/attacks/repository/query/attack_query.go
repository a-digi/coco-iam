// Package query is the read-only half of the attack-history
// repository — ip-attacks.db (a separate database from the main
// one), ip_attacks + ip_attack_targets tables.
package query

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	attacks_entity "github.com/a-digi/coco-iam/src/admin/security/attacks/entity"
	"github.com/a-digi/coco-iam/src/security/dbhandle"
)

// ErrNotFound signals no ip_attacks row exists for the given id.
var ErrNotFound = errors.New("ip-attack: not found")

// AttackQueryRepo reads through a *dbhandle.Handle rather than a raw
// *sql.DB, so the admin Attacks page keeps reading the live generation
// even across the archiver rotating ip-attacks.db out from under it —
// see plan/ip-attacks-db-archiving/plan.md.
type AttackQueryRepo struct {
	handle *dbhandle.Handle
}

func NewAttackQueryRepo(handle *dbhandle.Handle) *AttackQueryRepo {
	return &AttackQueryRepo{handle: handle}
}

// ListFilter narrows ListAttacks/CountAttacks — every string field is
// an exact match, empty means "don't filter on this".
type ListFilter struct {
	IP         string
	Tier       string
	ActiveOnly bool // ended_at IS NULL
	Limit      int
	Offset     int
}

func (f ListFilter) whereClause() (string, []interface{}) {
	clause := " WHERE 1=1"
	var args []interface{}
	if f.IP != "" {
		clause += " AND ip = ?"
		args = append(args, f.IP)
	}
	if f.Tier != "" {
		clause += " AND tier = ?"
		args = append(args, f.Tier)
	}
	if f.ActiveOnly {
		clause += " AND ended_at IS NULL"
	}
	return clause, args
}

// ListAttacks returns attack episodes newest-first, filtered and
// paginated per filter.
func (r *AttackQueryRepo) ListAttacks(filter ListFilter) ([]attacks_entity.Attack, error) {
	where, args := filter.whereClause()
	query := `SELECT id, ip, tier, started_at, last_seen_at, COALESCE(ended_at, ''), hit_count, ban_count
	          FROM ip_attacks` + where + ` ORDER BY started_at DESC LIMIT ? OFFSET ?`
	args = append(args, filter.Limit, filter.Offset)

	rows, err := r.handle.DB().Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("ip-attack: list: %w", err)
	}
	defer rows.Close()

	var out []attacks_entity.Attack
	for rows.Next() {
		var a attacks_entity.Attack
		if err := rows.Scan(&a.ID, &a.IP, &a.Tier, &a.StartedAt, &a.LastSeenAt, &a.EndedAt, &a.HitCount, &a.BanCount); err != nil {
			return nil, fmt.Errorf("ip-attack: scan: %w", err)
		}
		normalizeTimestamps(&a)
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ip-attack: rows: %w", err)
	}
	return out, nil
}

// CountAttacks returns how many rows match filter, ignoring
// filter.Limit/Offset — for the list endpoint's pagination total.
func (r *AttackQueryRepo) CountAttacks(filter ListFilter) (int, error) {
	where, args := filter.whereClause()
	var n int
	err := r.handle.DB().QueryRow(`SELECT COUNT(*) FROM ip_attacks`+where, args...).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("ip-attack: count: %w", err)
	}
	return n, nil
}

// FindAttack returns a single episode by id, or ErrNotFound.
func (r *AttackQueryRepo) FindAttack(id string) (*attacks_entity.Attack, error) {
	row := r.handle.DB().QueryRow(
		`SELECT id, ip, tier, started_at, last_seen_at, COALESCE(ended_at, ''), hit_count, ban_count
		 FROM ip_attacks WHERE id = ?`,
		id,
	)
	var a attacks_entity.Attack
	if err := row.Scan(&a.ID, &a.IP, &a.Tier, &a.StartedAt, &a.LastSeenAt, &a.EndedAt, &a.HitCount, &a.BanCount); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("ip-attack: find: %w", err)
	}
	normalizeTimestamps(&a)
	return &a, nil
}

// normalizeTimestamps rewrites StartedAt/LastSeenAt/EndedAt to a
// single consistent RFC3339 format. Needed because the sqlite driver
// auto-parses DATETIME-affinity columns but loses that affinity once
// a column is wrapped in COALESCE() (used here so a NULL ended_at
// scans as "" instead of requiring sql.NullString) — without this,
// EndedAt would come back in SQLite's raw "2006-01-02 15:04:05" form
// while StartedAt/LastSeenAt come back as RFC3339, a real inconsistency
// confirmed via a live API response during this step's verification.
func normalizeTimestamps(a *attacks_entity.Attack) {
	a.StartedAt = normalizeTimestamp(a.StartedAt)
	a.LastSeenAt = normalizeTimestamp(a.LastSeenAt)
	a.EndedAt = normalizeTimestamp(a.EndedAt)
}

func normalizeTimestamp(s string) string {
	if s == "" {
		return ""
	}
	for _, layout := range []string{time.RFC3339, time.RFC3339Nano, "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC().Format(time.RFC3339)
		}
	}
	return s
}

// ListTargets returns the per-endpoint hit-count breakdown for one
// episode, highest-hit-count first.
func (r *AttackQueryRepo) ListTargets(attackID string) ([]attacks_entity.AttackTarget, error) {
	rows, err := r.handle.DB().Query(
		`SELECT path, method, hit_count FROM ip_attack_targets WHERE attack_id = ? ORDER BY hit_count DESC`,
		attackID,
	)
	if err != nil {
		return nil, fmt.Errorf("ip-attack: list targets: %w", err)
	}
	defer rows.Close()

	var out []attacks_entity.AttackTarget
	for rows.Next() {
		var t attacks_entity.AttackTarget
		if err := rows.Scan(&t.Path, &t.Method, &t.HitCount); err != nil {
			return nil, fmt.Errorf("ip-attack: scan target: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ip-attack: target rows: %w", err)
	}
	return out, nil
}
