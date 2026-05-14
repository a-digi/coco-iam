package recovery

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/a-digi/coco-orm/orm"
)

// AdminStore persists admin recovery rows in admin_password_recoveries.
// The user_type column is not stored — all rows are implicitly admin.
type AdminStore struct {
	db *sql.DB
}

// NewAdminStore binds an AdminStore to the global users.db manager.
func NewAdminStore(dbm *orm.DatabaseManager) *AdminStore {
	return &AdminStore{db: dbm.Connector.DB}
}

// Insert persists a fresh admin recovery row.
func (s *AdminStore) Insert(r Row) error {
	if r.ID == "" {
		r.ID = newUUID()
	}
	_, err := s.db.Exec(
		`INSERT INTO admin_password_recoveries (id, user_id, token_hash, expires_at, created_at)
		 VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		r.ID, r.UserID, r.TokenHash,
		r.ExpiresAt.UTC().Format("2006-01-02 15:04:05"),
	)
	if err != nil {
		return fmt.Errorf("recovery: admin insert: %w", err)
	}
	return nil
}

// FindByTokenHash returns the row whose token hashes to the given
// value. The returned Row always has UserType == UserTypeAdmin.
func (s *AdminStore) FindByTokenHash(tokenHash string) (*Row, error) {
	row := s.db.QueryRow(
		`SELECT id, user_id, token_hash, expires_at, consumed_at, created_at
		 FROM admin_password_recoveries WHERE token_hash = ?`, tokenHash,
	)
	return scanRowWithType(row, UserTypeAdmin)
}

// ConsumeByID marks the row consumed and wipes sibling unconsumed rows
// for the same user. Runs in a transaction.
func (s *AdminStore) ConsumeByID(id string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("recovery: admin begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	var userID string
	err = tx.QueryRow(
		`SELECT user_id FROM admin_password_recoveries WHERE id = ? AND consumed_at IS NULL`, id,
	).Scan(&userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrAlreadyUsed
		}
		return fmt.Errorf("recovery: admin consume lookup: %w", err)
	}

	if _, err := tx.Exec(
		`UPDATE admin_password_recoveries SET consumed_at = CURRENT_TIMESTAMP WHERE id = ?`, id,
	); err != nil {
		return fmt.Errorf("recovery: admin consume update: %w", err)
	}
	if _, err := tx.Exec(
		`DELETE FROM admin_password_recoveries WHERE user_id = ? AND id <> ? AND consumed_at IS NULL`,
		userID, id,
	); err != nil {
		return fmt.Errorf("recovery: admin prune siblings: %w", err)
	}
	return tx.Commit()
}

// DeletePendingForUser wipes every unconsumed row for the admin user.
func (s *AdminStore) DeletePendingForUser(userID string) error {
	_, err := s.db.Exec(
		`DELETE FROM admin_password_recoveries WHERE user_id = ? AND consumed_at IS NULL`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("recovery: admin purge pending: %w", err)
	}
	return nil
}

// LatestPendingForUser returns the most recent unconsumed row, or nil.
func (s *AdminStore) LatestPendingForUser(userID string) (*Row, error) {
	row := s.db.QueryRow(
		`SELECT id, user_id, token_hash, expires_at, consumed_at, created_at
		 FROM admin_password_recoveries
		 WHERE user_id = ? AND consumed_at IS NULL
		 ORDER BY created_at DESC LIMIT 1`,
		userID,
	)
	r, err := scanRowWithType(row, UserTypeAdmin)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return r, nil
}
