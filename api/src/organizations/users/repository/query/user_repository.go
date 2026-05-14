package query

import (
	"database/sql"
	"errors"
)

// OrgUserQueryRepository performs read-only lookups against a per-org users DB.
// The caller is responsible for opening the correct per-org *sql.DB.
type OrgUserQueryRepository struct {
	db *sql.DB
}

func New(db *sql.DB) *OrgUserQueryRepository {
	return &OrgUserQueryRepository{db: db}
}

// ExistsByUsername reports whether any user in the org already holds the given
// username. Comparison is case-insensitive.
func (r *OrgUserQueryRepository) ExistsByUsername(username string) (bool, error) {
	var found int
	err := r.db.QueryRow(
		`SELECT 1 FROM users WHERE LOWER(username) = LOWER(?) LIMIT 1`, username,
	).Scan(&found)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// ExistsByEmailExcludingID reports whether any user other than excludeID
// already holds the given email. Pass excludeID="" to check without exclusion
// (create path). Comparison is case-insensitive.
func (r *OrgUserQueryRepository) ExistsByEmailExcludingID(email, excludeID string) (bool, error) {
	var found int
	var err error
	if excludeID == "" {
		err = r.db.QueryRow(
			`SELECT 1 FROM users WHERE LOWER(email) = LOWER(?) LIMIT 1`, email,
		).Scan(&found)
	} else {
		err = r.db.QueryRow(
			`SELECT 1 FROM users WHERE LOWER(email) = LOWER(?) AND id != ? LIMIT 1`, email, excludeID,
		).Scan(&found)
	}
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
