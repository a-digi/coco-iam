package query

import (
	"database/sql"
	"errors"
	"fmt"
)

// EventTemplateKey and EventAccountKey mirror
// api/src/organizations/mail/repository/query's own helpers of the same
// name (which in turn mirror api/src/mail/settings.EventTemplateKey/
// EventAccountKey) — kept as a small local duplicate rather than an
// import across domains, since applications and organizations are
// independent package boundaries; the string shape is a shared,
// stable convention, not organization-specific logic.
func EventTemplateKey(event string) string { return "event." + event + ".template" }
func EventAccountKey(event string) string  { return "event." + event + ".account" }

// Activation cadence keys — mirror api/src/mail/settings' constants.
const (
	KeyActivationTTLHours       = "activation.ttl_hours"
	KeyActivationResendCooldown = "activation.resend_cooldown_seconds"
)

// AppMailSettingsQueryRepo reads the app_mail_settings KV table scoped
// to a specific application.
type AppMailSettingsQueryRepo struct {
	db    *sql.DB
	appID string
}

func NewAppMailSettingsQueryRepo(db *sql.DB, appID string) *AppMailSettingsQueryRepo {
	return &AppMailSettingsQueryRepo{db: db, appID: appID}
}

// Get returns (value, true) if the key exists, otherwise ("", false) —
// mirrors settings.Store.Get exactly, including "empty value" meaning
// unset for callers doing cascade fallback.
func (r *AppMailSettingsQueryRepo) Get(key string) (string, bool, error) {
	var value string
	err := r.db.QueryRow(
		`SELECT value FROM app_mail_settings WHERE application_id = ? AND key = ?`, r.appID, key,
	).Scan(&value)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("app mail settings: get %q: %w", key, err)
	}
	return value, true, nil
}

func (r *AppMailSettingsQueryRepo) All() (map[string]string, error) {
	rows, err := r.db.Query(`SELECT key, value FROM app_mail_settings WHERE application_id = ?`, r.appID)
	if err != nil {
		return nil, fmt.Errorf("app mail settings: list: %w", err)
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, fmt.Errorf("app mail settings: scan: %w", err)
		}
		out[k] = v
	}
	return out, rows.Err()
}
