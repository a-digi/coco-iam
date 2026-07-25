// Package persistent is the write half of the admin MFA repository —
// main DB, admin_user_mfa + admin_user_mfa_recovery_codes tables.
package persistent

import (
	"database/sql"
	"fmt"
	"time"
)

type AdminMfaPersistentRepo struct {
	db *sql.DB
}

func NewAdminMfaPersistentRepo(db *sql.DB) *AdminMfaPersistentRepo {
	return &AdminMfaPersistentRepo{db: db}
}

// UpsertPendingSecret writes a freshly-generated secret for /enroll,
// replacing any prior pending or confirmed state — is_enabled is
// reset to false (a re-enroll before confirming, or re-enrolling
// after disabling, always starts from a clean slate) and the failure
// counter/lockout are cleared along with it.
func (r *AdminMfaPersistentRepo) UpsertPendingSecret(adminUserID, secretEnc string) error {
	_, err := r.db.Exec(
		`INSERT INTO admin_user_mfa
		   (admin_user_id, secret_enc, is_enabled, enrolled_at, confirmed_at, failed_attempts, locked_until, updated_at)
		 VALUES (?, ?, FALSE, CURRENT_TIMESTAMP, NULL, 0, NULL, CURRENT_TIMESTAMP)
		 ON CONFLICT(admin_user_id) DO UPDATE SET
		   secret_enc = excluded.secret_enc,
		   is_enabled = FALSE,
		   enrolled_at = CURRENT_TIMESTAMP,
		   confirmed_at = NULL,
		   failed_attempts = 0,
		   locked_until = NULL,
		   updated_at = CURRENT_TIMESTAMP`,
		adminUserID, secretEnc,
	)
	if err != nil {
		return fmt.Errorf("admin-mfa: upsert pending secret: %w", err)
	}
	return nil
}

// Confirm marks enrollment complete after a valid code was presented
// to /confirm.
func (r *AdminMfaPersistentRepo) Confirm(adminUserID string) error {
	res, err := r.db.Exec(
		`UPDATE admin_user_mfa
		    SET is_enabled = TRUE, confirmed_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		  WHERE admin_user_id = ?`,
		adminUserID,
	)
	if err != nil {
		return fmt.Errorf("admin-mfa: confirm: %w", err)
	}
	return requireRowAffected(res, "confirm")
}

// Disable removes MFA entirely for this admin — the row and every
// recovery code are deleted, not just flagged inactive, so a
// subsequent re-enroll starts completely fresh rather than reviving
// a disabled secret.
func (r *AdminMfaPersistentRepo) Disable(adminUserID string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("admin-mfa: disable: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`DELETE FROM admin_user_mfa_recovery_codes WHERE admin_user_id = ?`, adminUserID); err != nil {
		return fmt.Errorf("admin-mfa: disable: delete recovery codes: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM admin_user_mfa WHERE admin_user_id = ?`, adminUserID); err != nil {
		return fmt.Errorf("admin-mfa: disable: delete mfa row: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("admin-mfa: disable: commit: %w", err)
	}
	return nil
}

// RecordFailedAttempt increments the failure counter and, if
// lockUntil is non-nil, sets locked_until — the caller decides the
// threshold/backoff policy, this just persists the outcome.
func (r *AdminMfaPersistentRepo) RecordFailedAttempt(adminUserID string, lockUntil *time.Time) error {
	var lockedUntilArg interface{}
	if lockUntil != nil {
		lockedUntilArg = lockUntil.UTC().Format("2006-01-02 15:04:05")
	}
	_, err := r.db.Exec(
		`UPDATE admin_user_mfa
		    SET failed_attempts = failed_attempts + 1, locked_until = COALESCE(?, locked_until), updated_at = CURRENT_TIMESTAMP
		  WHERE admin_user_id = ?`,
		lockedUntilArg, adminUserID,
	)
	if err != nil {
		return fmt.Errorf("admin-mfa: record failed attempt: %w", err)
	}
	return nil
}

// ResetFailedAttempts clears the failure counter and any lockout —
// called after a successful code verification.
func (r *AdminMfaPersistentRepo) ResetFailedAttempts(adminUserID string) error {
	_, err := r.db.Exec(
		`UPDATE admin_user_mfa
		    SET failed_attempts = 0, locked_until = NULL, updated_at = CURRENT_TIMESTAMP
		  WHERE admin_user_id = ?`,
		adminUserID,
	)
	if err != nil {
		return fmt.Errorf("admin-mfa: reset failed attempts: %w", err)
	}
	return nil
}

// ReplaceRecoveryCodes deletes every existing recovery code for this
// admin and inserts the given set of (already-hashed) replacements —
// used by /confirm (first issuance) and the regenerate endpoint
// (invalidates everything previously issued).
func (r *AdminMfaPersistentRepo) ReplaceRecoveryCodes(adminUserID string, ids, codeHashes []string) error {
	if len(ids) != len(codeHashes) {
		return fmt.Errorf("admin-mfa: replace recovery codes: ids/hashes length mismatch")
	}
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("admin-mfa: replace recovery codes: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`DELETE FROM admin_user_mfa_recovery_codes WHERE admin_user_id = ?`, adminUserID); err != nil {
		return fmt.Errorf("admin-mfa: replace recovery codes: delete existing: %w", err)
	}
	for i := range ids {
		if _, err := tx.Exec(
			`INSERT INTO admin_user_mfa_recovery_codes (id, admin_user_id, code_hash) VALUES (?, ?, ?)`,
			ids[i], adminUserID, codeHashes[i],
		); err != nil {
			return fmt.Errorf("admin-mfa: replace recovery codes: insert: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("admin-mfa: replace recovery codes: commit: %w", err)
	}
	return nil
}

// MarkRecoveryCodeUsed consumes a single recovery code so it can
// never be replayed.
func (r *AdminMfaPersistentRepo) MarkRecoveryCodeUsed(id string) error {
	res, err := r.db.Exec(
		`UPDATE admin_user_mfa_recovery_codes SET used_at = CURRENT_TIMESTAMP WHERE id = ? AND used_at IS NULL`,
		id,
	)
	if err != nil {
		return fmt.Errorf("admin-mfa: mark recovery code used: %w", err)
	}
	return requireRowAffected(res, "mark recovery code used")
}

func requireRowAffected(res sql.Result, op string) error {
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("admin-mfa: %s: rows affected: %w", op, err)
	}
	if n == 0 {
		return fmt.Errorf("admin-mfa: %s: no matching row", op)
	}
	return nil
}
