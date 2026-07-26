// Package query is the read-only half of the admin IP ban/allowlist
// repository — main DB, ip_bans + ip_allowlist tables.
package query

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	security_entity "github.com/a-digi/coco-iam/src/admin/security/entity"
)

// ErrNotFound signals no ip_bans row exists for the given IP.
var ErrNotFound = errors.New("ip-ban: not found")

type IPBanQueryRepo struct {
	db *sql.DB
}

func NewIPBanQueryRepo(db *sql.DB) *IPBanQueryRepo {
	return &IPBanQueryRepo{db: db}
}

// ListBans returns every ban row, newest first — including already-
// expired ones the sweeper hasn't pruned yet.
func (r *IPBanQueryRepo) ListBans() ([]security_entity.IPBan, error) {
	rows, err := r.db.Query(
		`SELECT ip, tier, reason, banned_at, expires_at, hit_count, COALESCE(created_by, '')
		 FROM ip_bans ORDER BY banned_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("ip-ban: list: %w", err)
	}
	defer rows.Close()
	return scanBans(rows)
}

// ListActive returns only bans that have not yet expired as of now —
// used to hydrate the in-memory ban cache at startup.
func (r *IPBanQueryRepo) ListActive(now time.Time) ([]security_entity.IPBan, error) {
	rows, err := r.db.Query(
		`SELECT ip, tier, reason, banned_at, expires_at, hit_count, COALESCE(created_by, '')
		 FROM ip_bans WHERE expires_at > ? ORDER BY banned_at DESC`,
		now.UTC().Format("2006-01-02 15:04:05"),
	)
	if err != nil {
		return nil, fmt.Errorf("ip-ban: list active: %w", err)
	}
	defer rows.Close()
	return scanBans(rows)
}

// FindBan returns the ban row for a single IP, or ErrNotFound.
func (r *IPBanQueryRepo) FindBan(ip string) (*security_entity.IPBan, error) {
	row := r.db.QueryRow(
		`SELECT ip, tier, reason, banned_at, expires_at, hit_count, COALESCE(created_by, '')
		 FROM ip_bans WHERE ip = ?`,
		ip,
	)
	var b security_entity.IPBan
	if err := row.Scan(&b.IP, &b.Tier, &b.Reason, &b.BannedAt, &b.ExpiresAt, &b.HitCount, &b.CreatedBy); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("ip-ban: find: %w", err)
	}
	return &b, nil
}

func scanBans(rows *sql.Rows) ([]security_entity.IPBan, error) {
	var out []security_entity.IPBan
	for rows.Next() {
		var b security_entity.IPBan
		if err := rows.Scan(&b.IP, &b.Tier, &b.Reason, &b.BannedAt, &b.ExpiresAt, &b.HitCount, &b.CreatedBy); err != nil {
			return nil, fmt.Errorf("ip-ban: scan: %w", err)
		}
		out = append(out, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ip-ban: rows: %w", err)
	}
	return out, nil
}
