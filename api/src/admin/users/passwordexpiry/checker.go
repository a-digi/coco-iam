package passwordexpiry

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/a-digi/coco-iam/src/userrules"
)

type Checker struct {
	store *userrules.AdminStore
	db    *sql.DB
}

func New(store *userrules.AdminStore, db *sql.DB) *Checker {
	return &Checker{store: store, db: db}
}

// IsExpired returns true when the admin user's password has exceeded the
// configured expiry window. Returns (false, nil) when the feature is
// disabled, the user has no password row, or changed_at is NULL.
func (c *Checker) IsExpired(userID string) (bool, error) {
	rs, err := c.store.Get()
	if err != nil {
		return false, err
	}
	if rs.Password.ExpiryDays <= 0 {
		return false, nil
	}

	var changedAtStr sql.NullString
	err = c.db.QueryRow(
		`SELECT changed_at FROM admin_auth_password WHERE user_id = ? AND is_active = 1 LIMIT 1`,
		userID,
	).Scan(&changedAtStr)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	if !changedAtStr.Valid || changedAtStr.String == "" {
		return false, nil
	}

	changedAt, err := parseChangedAt(changedAtStr.String)
	if err != nil {
		return false, nil
	}

	expiry := changedAt.Add(time.Duration(rs.Password.ExpiryDays) * 24 * time.Hour)
	return time.Now().UTC().After(expiry), nil
}

// parseChangedAt handles the multiple datetime string formats that SQLite
// may return depending on how the value was inserted (space-separated vs RFC3339).
func parseChangedAt(s string) (time.Time, error) {
	formats := []string{
		"2006-01-02 15:04:05",
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse changed_at: %q", s)
}
