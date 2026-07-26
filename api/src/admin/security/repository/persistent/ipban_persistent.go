// Package persistent is the write half of the admin IP ban/allowlist
// repository — main DB, ip_bans + ip_allowlist tables.
package persistent

import (
	"database/sql"
	"fmt"
	"time"
)

type IPBanPersistentRepo struct {
	db *sql.DB
}

func NewIPBanPersistentRepo(db *sql.DB) *IPBanPersistentRepo {
	return &IPBanPersistentRepo{db: db}
}

// UpsertBan creates a new ban row, or — if this IP is already banned —
// extends it and increments hit_count, so a repeat offender climbs a
// single counter instead of accumulating duplicate rows. createdBy is
// nil for auto-bans (global/sensitive tier) and an admin_user_id for
// manual bans.
func (r *IPBanPersistentRepo) UpsertBan(ip, tier, reason string, expiresAt time.Time, createdBy *string) error {
	var createdByArg interface{}
	if createdBy != nil {
		createdByArg = *createdBy
	}
	_, err := r.db.Exec(
		`INSERT INTO ip_bans (ip, tier, reason, banned_at, expires_at, hit_count, created_by)
		 VALUES (?, ?, ?, CURRENT_TIMESTAMP, ?, 1, ?)
		 ON CONFLICT(ip) DO UPDATE SET
		   tier = excluded.tier,
		   reason = excluded.reason,
		   banned_at = CURRENT_TIMESTAMP,
		   expires_at = excluded.expires_at,
		   hit_count = ip_bans.hit_count + 1,
		   created_by = excluded.created_by`,
		ip, tier, reason, expiresAt.UTC().Format("2006-01-02 15:04:05"), createdByArg,
	)
	if err != nil {
		return fmt.Errorf("ip-ban: upsert: %w", err)
	}
	return nil
}

// DeleteBan removes a single IP's ban row (manual unban). Errors if
// no such ban exists, so the caller can 404.
func (r *IPBanPersistentRepo) DeleteBan(ip string) error {
	res, err := r.db.Exec(`DELETE FROM ip_bans WHERE ip = ?`, ip)
	if err != nil {
		return fmt.Errorf("ip-ban: delete: %w", err)
	}
	return requireRowAffected(res, "delete ban")
}

// DeleteExpired removes every ban whose expires_at has passed as of
// now, and reports how many rows were removed. Zero matches is the
// normal case (called on a fixed sweep interval) and is not an error.
func (r *IPBanPersistentRepo) DeleteExpired(now time.Time) (int64, error) {
	res, err := r.db.Exec(`DELETE FROM ip_bans WHERE expires_at < ?`, now.UTC().Format("2006-01-02 15:04:05"))
	if err != nil {
		return 0, fmt.Errorf("ip-ban: delete expired: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("ip-ban: delete expired: rows affected: %w", err)
	}
	return n, nil
}

func requireRowAffected(res sql.Result, op string) error {
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("ip-ban: %s: rows affected: %w", op, err)
	}
	if n == 0 {
		return fmt.Errorf("ip-ban: %s: no matching row", op)
	}
	return nil
}
