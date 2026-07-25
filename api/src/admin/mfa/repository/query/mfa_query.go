// Package query is the read-only half of the admin MFA repository —
// main DB, admin_user_mfa + admin_user_mfa_recovery_codes tables.
package query

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	mfa_entity "github.com/a-digi/coco-iam/src/admin/mfa/entity"
)

// ErrNotFound signals no admin_user_mfa row exists yet — the admin
// has never called /enroll.
var ErrNotFound = errors.New("admin-mfa: not found")

type AdminMfaQueryRepo struct {
	db *sql.DB
}

func NewAdminMfaQueryRepo(db *sql.DB) *AdminMfaQueryRepo {
	return &AdminMfaQueryRepo{db: db}
}

// FindByAdminUserID returns the current MFA row for the given admin,
// enrolled or not. Returns ErrNotFound if /enroll has never been
// called for this admin.
func (r *AdminMfaQueryRepo) FindByAdminUserID(adminUserID string) (*mfa_entity.AdminUserMfa, error) {
	row := r.db.QueryRow(
		`SELECT admin_user_id, secret_enc, is_enabled, enrolled_at, confirmed_at, failed_attempts, locked_until
		 FROM admin_user_mfa WHERE admin_user_id = ?`,
		adminUserID,
	)
	var m mfa_entity.AdminUserMfa
	var enrolledAt, confirmedAt, lockedUntil sql.NullString
	if err := row.Scan(
		&m.AdminUserID, &m.SecretEnc, &m.IsEnabled,
		&enrolledAt, &confirmedAt, &m.FailedAttempts, &lockedUntil,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("admin-mfa: scan: %w", err)
	}
	m.EnrolledAt = parseNullableTime(enrolledAt)
	m.ConfirmedAt = parseNullableTime(confirmedAt)
	m.LockedUntil = parseNullableTime(lockedUntil)
	return &m, nil
}

// CountUnusedRecoveryCodes returns how many recovery codes this admin
// still has available (used_at IS NULL).
func (r *AdminMfaQueryRepo) CountUnusedRecoveryCodes(adminUserID string) (int, error) {
	var n int
	err := r.db.QueryRow(
		`SELECT COUNT(*) FROM admin_user_mfa_recovery_codes WHERE admin_user_id = ? AND used_at IS NULL`,
		adminUserID,
	).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("admin-mfa: count recovery codes: %w", err)
	}
	return n, nil
}

// FindUnusedRecoveryCodes returns every not-yet-consumed recovery
// code hash for this admin, for the caller to bcrypt-compare a
// submitted code against.
func (r *AdminMfaQueryRepo) FindUnusedRecoveryCodes(adminUserID string) ([]mfa_entity.RecoveryCode, error) {
	rows, err := r.db.Query(
		`SELECT id, code_hash FROM admin_user_mfa_recovery_codes WHERE admin_user_id = ? AND used_at IS NULL`,
		adminUserID,
	)
	if err != nil {
		return nil, fmt.Errorf("admin-mfa: query recovery codes: %w", err)
	}
	defer rows.Close()

	var out []mfa_entity.RecoveryCode
	for rows.Next() {
		var c mfa_entity.RecoveryCode
		if err := rows.Scan(&c.ID, &c.CodeHash); err != nil {
			return nil, fmt.Errorf("admin-mfa: scan recovery code: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("admin-mfa: recovery code rows: %w", err)
	}
	return out, nil
}

func parseNullableTime(s sql.NullString) *time.Time {
	if !s.Valid {
		return nil
	}
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05",
	} {
		if t, err := time.Parse(layout, s.String); err == nil {
			t = t.UTC()
			return &t
		}
	}
	return nil
}
