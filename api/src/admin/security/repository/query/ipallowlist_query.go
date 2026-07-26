package query

import (
	"database/sql"
	"errors"
	"fmt"

	security_entity "github.com/a-digi/coco-iam/src/admin/security/entity"
)

// ErrAllowlistEntryNotFound signals no ip_allowlist row exists for the
// given IP.
var ErrAllowlistEntryNotFound = errors.New("ip-allowlist: not found")

type IPAllowlistQueryRepo struct {
	db *sql.DB
}

func NewIPAllowlistQueryRepo(db *sql.DB) *IPAllowlistQueryRepo {
	return &IPAllowlistQueryRepo{db: db}
}

// ListAllowlist returns every allowlisted IP, newest first.
func (r *IPAllowlistQueryRepo) ListAllowlist() ([]security_entity.IPAllowlistEntry, error) {
	rows, err := r.db.Query(
		`SELECT ip, COALESCE(note, ''), created_at, created_by FROM ip_allowlist ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("ip-allowlist: list: %w", err)
	}
	defer rows.Close()

	var out []security_entity.IPAllowlistEntry
	for rows.Next() {
		var e security_entity.IPAllowlistEntry
		if err := rows.Scan(&e.IP, &e.Note, &e.CreatedAt, &e.CreatedBy); err != nil {
			return nil, fmt.Errorf("ip-allowlist: scan: %w", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ip-allowlist: rows: %w", err)
	}
	return out, nil
}

// FindAllowlistEntry returns the allowlist row for a single IP, or
// ErrAllowlistEntryNotFound.
func (r *IPAllowlistQueryRepo) FindAllowlistEntry(ip string) (*security_entity.IPAllowlistEntry, error) {
	row := r.db.QueryRow(
		`SELECT ip, COALESCE(note, ''), created_at, created_by FROM ip_allowlist WHERE ip = ?`,
		ip,
	)
	var e security_entity.IPAllowlistEntry
	if err := row.Scan(&e.IP, &e.Note, &e.CreatedAt, &e.CreatedBy); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAllowlistEntryNotFound
		}
		return nil, fmt.Errorf("ip-allowlist: find: %w", err)
	}
	return &e, nil
}
