package persistent

import (
	"database/sql"
	"fmt"
)

type OrgMailSettingsPersistentRepo struct {
	db *sql.DB
}

func NewOrgMailSettingsPersistentRepo(db *sql.DB) *OrgMailSettingsPersistentRepo {
	return &OrgMailSettingsPersistentRepo{db: db}
}

// Set upserts a key. An empty value clears the override while keeping
// the row — mirrors settings.Store.Set.
func (r *OrgMailSettingsPersistentRepo) Set(key, value string) error {
	_, err := r.db.Exec(
		`INSERT INTO org_mail_settings (key, value, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP`,
		key, value,
	)
	if err != nil {
		return fmt.Errorf("org mail settings: set %q: %w", key, err)
	}
	return nil
}

func (r *OrgMailSettingsPersistentRepo) SetMany(kv map[string]string) error {
	for k, v := range kv {
		if err := r.Set(k, v); err != nil {
			return err
		}
	}
	return nil
}
