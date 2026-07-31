package query

import (
	"database/sql"
	"errors"
	"fmt"
)

// EventTemplateKey and EventAccountKey mirror
// api/src/mail/settings.EventTemplateKey/EventAccountKey exactly, so the
// same event catalog (api/src/mail/settings.EventCatalog) applies at
// every tier.
func EventTemplateKey(event string) string { return "event." + event + ".template" }
func EventAccountKey(event string) string  { return "event." + event + ".account" }

// Activation cadence keys — mirror api/src/mail/settings' constants.
const (
	KeyActivationTTLHours       = "activation.ttl_hours"
	KeyActivationResendCooldown = "activation.resend_cooldown_seconds"
)

// OrgMailSettingsQueryRepo reads the org_mail_settings KV table from a
// specific org's users.db.
type OrgMailSettingsQueryRepo struct {
	db *sql.DB
}

func NewOrgMailSettingsQueryRepo(db *sql.DB) *OrgMailSettingsQueryRepo {
	return &OrgMailSettingsQueryRepo{db: db}
}

// Get returns (value, true) if the key exists, otherwise ("", false) —
// mirrors settings.Store.Get exactly, including "empty value" meaning
// unset for callers doing cascade fallback.
func (r *OrgMailSettingsQueryRepo) Get(key string) (string, bool, error) {
	var value string
	err := r.db.QueryRow(`SELECT value FROM org_mail_settings WHERE key = ?`, key).Scan(&value)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("org mail settings: get %q: %w", key, err)
	}
	return value, true, nil
}

func (r *OrgMailSettingsQueryRepo) All() (map[string]string, error) {
	rows, err := r.db.Query(`SELECT key, value FROM org_mail_settings`)
	if err != nil {
		return nil, fmt.Errorf("org mail settings: list: %w", err)
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, fmt.Errorf("org mail settings: scan: %w", err)
		}
		out[k] = v
	}
	return out, rows.Err()
}
