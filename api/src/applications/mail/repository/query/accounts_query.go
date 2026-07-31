// Package query holds read-only repositories over an application's
// SMTP/template/settings mail tables — mirrors
// api/src/organizations/mail/repository/query field-for-field, one tier
// deeper: every query is scoped by application_id since applications
// share their org's users.db (unlike the org tier, which is scoped
// implicitly by living in its own DB). See
// plan/org-app-email-settings/plan.md.
package query

import (
	"database/sql"
	"errors"
	"fmt"
)

// ErrNotFound is returned by Get / GetByName when no row matches.
var ErrNotFound = errors.New("app mail account: not found")

// ErrNoActive is returned by GetActive when the app has no active
// account of its own — callers fall back to the org's, then the
// global, active account.
var ErrNoActive = errors.New("app mail account: no active account")

// AppMailAccount is the full row, including Password — internal to the
// query/persistent packages. Handlers redact Password before returning
// entity.AppMailAccountResponse.
type AppMailAccount struct {
	ID        string
	Name      string
	Host      string
	Port      int
	Username  string
	Password  string
	FromName  string
	FromEmail string
	UseTLS    bool
	IsActive  bool
	CreatedAt string
	UpdatedAt string
}

// AppMailAccountsQueryRepo reads app_mail_smtp_accounts from a
// specific application's own row set, scoped by appID, inside the org's
// users.db that hosts it. Constructed per-request against the org DB
// resolved from the application's id.
type AppMailAccountsQueryRepo struct {
	db    *sql.DB
	appID string
}

func NewAppMailAccountsQueryRepo(db *sql.DB, appID string) *AppMailAccountsQueryRepo {
	return &AppMailAccountsQueryRepo{db: db, appID: appID}
}

func (r *AppMailAccountsQueryRepo) Get(id string) (*AppMailAccount, error) {
	row := r.db.QueryRow(
		`SELECT id, name, host, port, username, password, from_name, from_email, use_tls, is_active, created_at, updated_at
		 FROM app_mail_smtp_accounts WHERE id = ? AND application_id = ?`, id, r.appID,
	)
	return scanAccountRow(row)
}

func (r *AppMailAccountsQueryRepo) GetByName(name string) (*AppMailAccount, error) {
	row := r.db.QueryRow(
		`SELECT id, name, host, port, username, password, from_name, from_email, use_tls, is_active, created_at, updated_at
		 FROM app_mail_smtp_accounts WHERE name = ? AND application_id = ?`, name, r.appID,
	)
	return scanAccountRow(row)
}

// GetActive returns the application's own active account, or
// ErrNoActive if none is set — the caller (the scoped resolver) falls
// back to the org's, then the global, active account in that case.
func (r *AppMailAccountsQueryRepo) GetActive() (*AppMailAccount, error) {
	row := r.db.QueryRow(
		`SELECT id, name, host, port, username, password, from_name, from_email, use_tls, is_active, created_at, updated_at
		 FROM app_mail_smtp_accounts WHERE application_id = ? AND is_active = TRUE LIMIT 1`, r.appID,
	)
	out, err := scanAccountRow(row)
	if err != nil && errors.Is(err, ErrNotFound) {
		return nil, ErrNoActive
	}
	return out, err
}

func (r *AppMailAccountsQueryRepo) Exists(name string) (bool, error) {
	var n int
	if err := r.db.QueryRow(
		`SELECT COUNT(1) FROM app_mail_smtp_accounts WHERE name = ? AND application_id = ?`, name, r.appID,
	).Scan(&n); err != nil {
		return false, fmt.Errorf("app mail account: exists %q: %w", name, err)
	}
	return n > 0, nil
}

// List returns every account for this application, ordered by name.
func (r *AppMailAccountsQueryRepo) List() ([]AppMailAccount, error) {
	rows, err := r.db.Query(
		`SELECT id, name, host, port, username, password, from_name, from_email, use_tls, is_active, created_at, updated_at
		 FROM app_mail_smtp_accounts WHERE application_id = ? ORDER BY name ASC`, r.appID,
	)
	if err != nil {
		return nil, fmt.Errorf("app mail account: list: %w", err)
	}
	defer rows.Close()

	out := make([]AppMailAccount, 0)
	for rows.Next() {
		a, serr := scanAccountRow(rows)
		if serr != nil {
			return nil, serr
		}
		out = append(out, *a)
	}
	return out, rows.Err()
}

type scannableAccount interface {
	Scan(dest ...interface{}) error
}

func scanAccountRow(r scannableAccount) (*AppMailAccount, error) {
	var a AppMailAccount
	err := r.Scan(
		&a.ID, &a.Name, &a.Host, &a.Port, &a.Username, &a.Password,
		&a.FromName, &a.FromEmail, &a.UseTLS, &a.IsActive,
		&a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("app mail account: scan: %w", err)
	}
	return &a, nil
}
