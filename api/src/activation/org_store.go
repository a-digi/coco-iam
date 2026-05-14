package activation

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/a-digi/coco-iam/src/organizations/users/dbregistry"
	"github.com/a-digi/coco-iam/src/orgrouter"
)

// OrgStore is the CRUD wrapper over the per-org user_activations table.
// Each organization has its own users.db; FindByTokenHash scans all known orgs.
type OrgStore struct {
	orgIDs func() []string
	openDB func(orgID string) (*sql.DB, error)
}

// NewOrgStore constructs an OrgStore backed by the given per-org registry.
func NewOrgStore(reg *dbregistry.OrgUserDBRegistry) *OrgStore {
	return &OrgStore{
		orgIDs: reg.KnownOrgIDs,
		openDB: func(orgID string) (*sql.DB, error) {
			return orgrouter.ForOrg(reg, orgID)
		},
	}
}

// Insert persists a new activation row into the provided per-org DB.
// ID is generated if empty.
func (s *OrgStore) Insert(r Row, orgDB *sql.DB) error {
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
	_, err := orgDB.Exec(
		`INSERT INTO user_activations
		 (id, user_id, token_hash, temp_password_hash, expires_at, created_at,
		  redirect_organization_slug, redirect_workspace_slug, redirect_application_client_id)
		 VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP, ?, ?, ?)`,
		r.ID, r.UserID, r.TokenHash, r.TempPasswordHash,
		r.ExpiresAt.UTC().Format("2006-01-02 15:04:05"),
		orgSlug, wsSlug, clientID,
	)
	if err != nil {
		return fmt.Errorf("activation org: insert: %w", err)
	}
	return nil
}

// FindByTokenHash scans all known org DBs and returns the row plus the DB
// that holds it. Returns ErrNotFound if no org contains a matching row.
func (s *OrgStore) FindByTokenHash(tokenHash string) (*Row, *sql.DB, error) {
	for _, orgID := range s.orgIDs() {
		orgDB, err := s.openDB(orgID)
		if err != nil {
			continue
		}
		r := orgDB.QueryRow(
			`SELECT id, user_id, token_hash, temp_password_hash, expires_at, consumed_at, created_at,
			        redirect_organization_slug, redirect_workspace_slug, redirect_application_client_id
			 FROM user_activations WHERE token_hash = ?`, tokenHash,
		)
		row, err := scanRow(r, UserTypeUser)
		if err == nil {
			return row, orgDB, nil
		}
	}
	return nil, nil, ErrNotFound
}

// ConsumeByID marks the row as consumed and deletes sibling unconsumed rows
// for the same user. Runs in a transaction against the provided org DB.
func (s *OrgStore) ConsumeByID(id string, orgDB *sql.DB) error {
	tx, err := orgDB.Begin()
	if err != nil {
		return fmt.Errorf("activation org: begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	var userID string
	err = tx.QueryRow(
		`SELECT user_id FROM user_activations WHERE id = ? AND consumed_at IS NULL`, id,
	).Scan(&userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrAlreadyUsed
		}
		return fmt.Errorf("activation org: consume lookup: %w", err)
	}

	if _, err := tx.Exec(
		`UPDATE user_activations SET consumed_at = CURRENT_TIMESTAMP WHERE id = ?`, id,
	); err != nil {
		return fmt.Errorf("activation org: consume update: %w", err)
	}
	if _, err := tx.Exec(
		`DELETE FROM user_activations WHERE user_id = ? AND id <> ? AND consumed_at IS NULL`,
		userID, id,
	); err != nil {
		return fmt.Errorf("activation org: prune siblings: %w", err)
	}
	return tx.Commit()
}

// DeletePendingForUser removes every unconsumed activation for the user
// from the provided org DB.
func (s *OrgStore) DeletePendingForUser(userID string, orgDB *sql.DB) error {
	_, err := orgDB.Exec(
		`DELETE FROM user_activations WHERE user_id = ? AND consumed_at IS NULL`, userID,
	)
	if err != nil {
		return fmt.Errorf("activation org: purge pending: %w", err)
	}
	return nil
}

// HasConsumedActivation returns true when the user has at least one activation
// row with consumed_at IS NOT NULL, meaning they completed the activation flow.
func (s *OrgStore) HasConsumedActivation(userID string, orgDB *sql.DB) (bool, error) {
	var n int
	err := orgDB.QueryRow(
		`SELECT COUNT(*) FROM user_activations
		 WHERE user_id = ? AND consumed_at IS NOT NULL`, userID,
	).Scan(&n)
	return n > 0, err
}

// LatestPendingForUser returns the most recently-created unconsumed row for
// the user in the provided org DB, or nil if none.
func (s *OrgStore) LatestPendingForUser(userID string, orgDB *sql.DB) (*Row, error) {
	r := orgDB.QueryRow(
		`SELECT id, user_id, token_hash, temp_password_hash, expires_at, consumed_at, created_at,
		        redirect_organization_slug, redirect_workspace_slug, redirect_application_client_id
		 FROM user_activations
		 WHERE user_id = ? AND consumed_at IS NULL
		 ORDER BY created_at DESC LIMIT 1`, userID,
	)
	row, err := scanRow(r, UserTypeUser)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return row, nil
}
