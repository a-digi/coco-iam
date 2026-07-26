package persistent

import (
	"database/sql"
	"fmt"
)

type IPAllowlistPersistentRepo struct {
	db *sql.DB
}

func NewIPAllowlistPersistentRepo(db *sql.DB) *IPAllowlistPersistentRepo {
	return &IPAllowlistPersistentRepo{db: db}
}

// InsertAllowlistEntry adds ip to the allowlist, or replaces the
// existing entry's note/creator if it's already present.
func (r *IPAllowlistPersistentRepo) InsertAllowlistEntry(ip, note, createdBy string) error {
	_, err := r.db.Exec(
		`INSERT INTO ip_allowlist (ip, note, created_at, created_by)
		 VALUES (?, ?, CURRENT_TIMESTAMP, ?)
		 ON CONFLICT(ip) DO UPDATE SET
		   note = excluded.note,
		   created_by = excluded.created_by`,
		ip, note, createdBy,
	)
	if err != nil {
		return fmt.Errorf("ip-allowlist: insert: %w", err)
	}
	return nil
}

// DeleteAllowlistEntry removes an IP from the allowlist. Errors if no
// such entry exists, so the caller can 404.
func (r *IPAllowlistPersistentRepo) DeleteAllowlistEntry(ip string) error {
	res, err := r.db.Exec(`DELETE FROM ip_allowlist WHERE ip = ?`, ip)
	if err != nil {
		return fmt.Errorf("ip-allowlist: delete: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("ip-allowlist: delete: rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("ip-allowlist: delete: no matching row")
	}
	return nil
}
