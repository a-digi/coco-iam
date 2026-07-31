package persistent

import (
	"database/sql"
	"fmt"
)

type AppMailSettingsPersistentRepo struct {
	db    *sql.DB
	appID string
}

func NewAppMailSettingsPersistentRepo(db *sql.DB, appID string) *AppMailSettingsPersistentRepo {
	return &AppMailSettingsPersistentRepo{db: db, appID: appID}
}

// Set upserts a key. An empty value clears the override while keeping
// the row — mirrors the org tier's Set.
func (r *AppMailSettingsPersistentRepo) Set(key, value string) error {
	_, err := r.db.Exec(
		`INSERT INTO app_mail_settings (application_id, key, value, updated_at) VALUES (?, ?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(application_id, key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP`,
		r.appID, key, value,
	)
	if err != nil {
		return fmt.Errorf("app mail settings: set %q: %w", key, err)
	}
	return nil
}

func (r *AppMailSettingsPersistentRepo) SetMany(kv map[string]string) error {
	for k, v := range kv {
		if err := r.Set(k, v); err != nil {
			return err
		}
	}
	return nil
}
