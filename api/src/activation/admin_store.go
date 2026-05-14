package activation

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/a-digi/coco-orm/orm"
)

// AdminStore is the CRUD wrapper over admin_activations in users.db.
type AdminStore struct {
	db *sql.DB
}

// NewAdminStore binds an AdminStore to the main DatabaseManager.
func NewAdminStore(dbm *orm.DatabaseManager) *AdminStore {
	return &AdminStore{db: dbm.Connector.DB}
}

// Insert persists a new admin activation row. ID is generated if empty.
func (s *AdminStore) Insert(r Row) error {
	if r.ID == "" {
		r.ID = newUUID()
	}
	var orgSlug, wsSlug, clientID interface{}
	if r.RedirectOrgSlug != "" {
		orgSlug = r.RedirectOrgSlug
	}
	if r.RedirectWorkspaceSlug != "" {
		wsSlug = r.RedirectWorkspaceSlug
	}
	if r.RedirectClientID != "" {
		clientID = r.RedirectClientID
	}
	_, err := s.db.Exec(
		`INSERT INTO admin_activations
		 (id, user_id, token_hash, temp_password_hash, expires_at, created_at,
		  redirect_organization_slug, redirect_workspace_slug, redirect_application_client_id)
		 VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP, ?, ?, ?)`,
		r.ID, r.UserID, r.TokenHash, r.TempPasswordHash,
		r.ExpiresAt.UTC().Format("2006-01-02 15:04:05"),
		orgSlug, wsSlug, clientID,
	)
	if err != nil {
		return fmt.Errorf("activation admin: insert: %w", err)
	}
	return nil
}

// FindByTokenHash returns the admin activation row whose token hashes to the
// supplied value. Expired and consumed rows are still returned.
func (s *AdminStore) FindByTokenHash(tokenHash string) (*Row, error) {
	r := s.db.QueryRow(
		`SELECT id, user_id, token_hash, temp_password_hash, expires_at, consumed_at, created_at,
		        redirect_organization_slug, redirect_workspace_slug, redirect_application_client_id
		 FROM admin_activations WHERE token_hash = ?`, tokenHash,
	)
	return scanRow(r, UserTypeAdmin)
}

// ConsumeByID marks the row as consumed and deletes sibling unconsumed rows
// for the same admin user. Runs in a transaction.
func (s *AdminStore) ConsumeByID(id string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("activation admin: begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	var userID string
	err = tx.QueryRow(
		`SELECT user_id FROM admin_activations WHERE id = ? AND consumed_at IS NULL`, id,
	).Scan(&userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrAlreadyUsed
		}
		return fmt.Errorf("activation admin: consume lookup: %w", err)
	}

	if _, err := tx.Exec(
		`UPDATE admin_activations SET consumed_at = CURRENT_TIMESTAMP WHERE id = ?`, id,
	); err != nil {
		return fmt.Errorf("activation admin: consume update: %w", err)
	}
	if _, err := tx.Exec(
		`DELETE FROM admin_activations WHERE user_id = ? AND id <> ? AND consumed_at IS NULL`,
		userID, id,
	); err != nil {
		return fmt.Errorf("activation admin: prune siblings: %w", err)
	}
	return tx.Commit()
}

// DeletePendingForUser removes every unconsumed activation for the admin user.
func (s *AdminStore) DeletePendingForUser(userID string) error {
	_, err := s.db.Exec(
		`DELETE FROM admin_activations WHERE user_id = ? AND consumed_at IS NULL`, userID,
	)
	if err != nil {
		return fmt.Errorf("activation admin: purge pending: %w", err)
	}
	return nil
}

// LatestPendingForUser returns the most recently-created unconsumed row for
// the admin user, or nil if none. Used by Resend to enforce the cooldown.
func (s *AdminStore) LatestPendingForUser(userID string) (*Row, error) {
	r := s.db.QueryRow(
		`SELECT id, user_id, token_hash, temp_password_hash, expires_at, consumed_at, created_at,
		        redirect_organization_slug, redirect_workspace_slug, redirect_application_client_id
		 FROM admin_activations
		 WHERE user_id = ? AND consumed_at IS NULL
		 ORDER BY created_at DESC LIMIT 1`, userID,
	)
	row, err := scanRow(r, UserTypeAdmin)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return row, nil
}
