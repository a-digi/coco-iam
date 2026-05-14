package adminpwnotify

import (
	"database/sql"
	"encoding/json"
	"errors"
)

type PrefsRepository struct {
	db *sql.DB
}

func NewPrefsRepository(db *sql.DB) *PrefsRepository {
	return &PrefsRepository{db: db}
}

func (r *PrefsRepository) Get(userID string) ([]int, error) {
	var raw string
	err := r.db.QueryRow(
		`SELECT notify_days FROM admin_password_notify_prefs WHERE user_id = ? LIMIT 1`,
		userID,
	).Scan(&raw)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []int{}, nil
		}
		return nil, err
	}
	var days []int
	if err := json.Unmarshal([]byte(raw), &days); err != nil {
		return []int{}, nil
	}
	return days, nil
}

func (r *PrefsRepository) Upsert(userID string, days []int) error {
	b, err := json.Marshal(days)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(
		`INSERT INTO admin_password_notify_prefs (user_id, notify_days, updated_at)
		 VALUES (?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(user_id) DO UPDATE SET notify_days = excluded.notify_days, updated_at = CURRENT_TIMESTAMP`,
		userID, string(b),
	)
	return err
}
