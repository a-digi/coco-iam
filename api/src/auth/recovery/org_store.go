package recovery

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/a-digi/coco-iam/src/organizations/users/dbregistry"
	"github.com/a-digi/coco-iam/src/orgrouter"
)

// OrgStore persists org-user recovery rows in per-org password_recoveries.
// The user_type column is not stored — all rows are implicitly UserTypeUser.
type OrgStore struct {
	orgIDs func() []string
	openDB func(orgID string) (*sql.DB, error)
}

// NewOrgStore wires an OrgStore to the per-org registry.
func NewOrgStore(reg *dbregistry.OrgUserDBRegistry) *OrgStore {
	return &OrgStore{
		orgIDs: reg.KnownOrgIDs,
		openDB: func(orgID string) (*sql.DB, error) { return orgrouter.ForOrg(reg, orgID) },
	}
}

// Insert persists a fresh recovery row into the given per-org DB.
func (s *OrgStore) Insert(r Row, orgDB *sql.DB) error {
	if r.ID == "" {
		r.ID = newUUID()
	}
	_, err := orgDB.Exec(
		`INSERT INTO password_recoveries (id, user_id, token_hash, expires_at, created_at)
		 VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		r.ID, r.UserID, r.TokenHash,
		r.ExpiresAt.UTC().Format("2006-01-02 15:04:05"),
	)
	if err != nil {
		return fmt.Errorf("recovery: org insert: %w", err)
	}
	return nil
}

// FindByTokenHash scans all known per-org DBs for the token hash.
// Returns the Row, the matching DB, and nil on success.
// The returned Row always has UserType == UserTypeUser.
func (s *OrgStore) FindByTokenHash(tokenHash string) (*Row, *sql.DB, error) {
	for _, orgID := range s.orgIDs() {
		odb, err := s.openDB(orgID)
		if err != nil || odb == nil {
			continue
		}
		r, err := scanRowWithType(odb.QueryRow(
			`SELECT id, user_id, token_hash, expires_at, consumed_at, created_at
			 FROM password_recoveries WHERE token_hash = ?`, tokenHash,
		), UserTypeUser)
		if err == nil {
			return r, odb, nil
		}
		if !errors.Is(err, ErrNotFound) {
			return nil, nil, err
		}
	}
	return nil, nil, ErrNotFound
}

// ConsumeByID marks the row consumed and prunes sibling unconsumed
// rows for the same user within orgDB. Runs in a transaction.
func (s *OrgStore) ConsumeByID(id string, orgDB *sql.DB) error {
	tx, err := orgDB.Begin()
	if err != nil {
		return fmt.Errorf("recovery: org begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	var userID string
	err = tx.QueryRow(
		`SELECT user_id FROM password_recoveries WHERE id = ? AND consumed_at IS NULL`, id,
	).Scan(&userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrAlreadyUsed
		}
		return fmt.Errorf("recovery: org consume lookup: %w", err)
	}

	if _, err := tx.Exec(
		`UPDATE password_recoveries SET consumed_at = CURRENT_TIMESTAMP WHERE id = ?`, id,
	); err != nil {
		return fmt.Errorf("recovery: org consume update: %w", err)
	}
	if _, err := tx.Exec(
		`DELETE FROM password_recoveries WHERE user_id = ? AND id <> ? AND consumed_at IS NULL`,
		userID, id,
	); err != nil {
		return fmt.Errorf("recovery: org prune siblings: %w", err)
	}
	return tx.Commit()
}

// DeletePendingForUser wipes every unconsumed row for the user within orgDB.
func (s *OrgStore) DeletePendingForUser(userID string, orgDB *sql.DB) error {
	_, err := orgDB.Exec(
		`DELETE FROM password_recoveries WHERE user_id = ? AND consumed_at IS NULL`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("recovery: org purge pending: %w", err)
	}
	return nil
}

// LatestPendingForUser returns the most recent unconsumed row within orgDB, or nil.
func (s *OrgStore) LatestPendingForUser(userID string, orgDB *sql.DB) (*Row, error) {
	row := orgDB.QueryRow(
		`SELECT id, user_id, token_hash, expires_at, consumed_at, created_at
		 FROM password_recoveries
		 WHERE user_id = ? AND consumed_at IS NULL
		 ORDER BY created_at DESC LIMIT 1`,
		userID,
	)
	r, err := scanRowWithType(row, UserTypeUser)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return r, nil
}
