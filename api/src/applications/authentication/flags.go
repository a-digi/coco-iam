package authentication

import "database/sql"

// passwordLoginAllowed reads the application's
// allow_password_login column. Missing column / row / DB error
// all collapse to true so an environment that hasn't run the
// migration yet doesn't accidentally lock password login out.
// The invariant we care about is "when the admin flips this
// OFF, password login really is off" — false positives here
// are unsafe, false negatives (returning true when the row is
// missing) just preserve legacy behaviour.
func passwordLoginAllowed(db *sql.DB, appID string) bool {
	if db == nil || appID == "" {
		return true
	}
	var allow sql.NullBool
	err := db.QueryRow(
		`SELECT allow_password_login FROM applications WHERE id = ? LIMIT 1`,
		appID,
	).Scan(&allow)
	if err != nil {
		return true
	}
	if !allow.Valid {
		return true
	}
	return allow.Bool
}
