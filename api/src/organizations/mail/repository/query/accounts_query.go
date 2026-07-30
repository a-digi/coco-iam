// Package query holds read-only repositories over an organization's
// own users.db mail tables (org_mail_smtp_accounts, org_mail_templates,
// org_mail_settings) — mirrors api/src/mail/accounts, api/src/mail/template,
// and api/src/mail/settings field-for-field, scoped by living in the
// org DB instead of the global mail.db. See
// plan/org-app-email-settings/plan.md.
package query

import (
	"database/sql"
	"errors"
	"fmt"
)

// ErrNotFound is returned by Get / GetByName when no row matches.
var ErrNotFound = errors.New("org mail account: not found")

// ErrNoActive is returned by GetActive when the org has no active
// account of its own — callers fall back to the global active account.
var ErrNoActive = errors.New("org mail account: no active account")

// OrgMailAccount is the full row, including Password — internal to the
// query/persistent packages. Handlers redact Password before returning
// entity.OrgMailAccountResponse.
type OrgMailAccount struct {
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

// OrgMailAccountsQueryRepo reads org_mail_smtp_accounts from a specific
// org's users.db. Constructed per-request against the org DB resolved
// from the {id} path segment — unlike the global mail package, this
// isn't bound once at boot.
type OrgMailAccountsQueryRepo struct {
	db *sql.DB
}

func NewOrgMailAccountsQueryRepo(db *sql.DB) *OrgMailAccountsQueryRepo {
	return &OrgMailAccountsQueryRepo{db: db}
}

func (r *OrgMailAccountsQueryRepo) Get(id string) (*OrgMailAccount, error) {
	row := r.db.QueryRow(
		`SELECT id, name, host, port, username, password, from_name, from_email, use_tls, is_active, created_at, updated_at
		 FROM org_mail_smtp_accounts WHERE id = ?`, id,
	)
	return scanAccountRow(row)
}

func (r *OrgMailAccountsQueryRepo) GetByName(name string) (*OrgMailAccount, error) {
	row := r.db.QueryRow(
		`SELECT id, name, host, port, username, password, from_name, from_email, use_tls, is_active, created_at, updated_at
		 FROM org_mail_smtp_accounts WHERE name = ?`, name,
	)
	return scanAccountRow(row)
}

// GetActive returns the org own active account, or ErrNoActive if none
// is set — the caller (the scoped resolver) falls back to the global
// active account in that case.
func (r *OrgMailAccountsQueryRepo) GetActive() (*OrgMailAccount, error) {
	row := r.db.QueryRow(
		`SELECT id, name, host, port, username, password, from_name, from_email, use_tls, is_active, created_at, updated_at
		 FROM org_mail_smtp_accounts WHERE is_active = TRUE LIMIT 1`,
	)
	out, err := scanAccountRow(row)
	if err != nil && errors.Is(err, ErrNotFound) {
		return nil, ErrNoActive
	}
	return out, err
}

func (r *OrgMailAccountsQueryRepo) Exists(name string) (bool, error) {
	var n int
	if err := r.db.QueryRow(`SELECT COUNT(1) FROM org_mail_smtp_accounts WHERE name = ?`, name).Scan(&n); err != nil {
		return false, fmt.Errorf("org mail account: exists %q: %w", name, err)
	}
	return n > 0, nil
}

// List returns every account for this org, ordered by name.
func (r *OrgMailAccountsQueryRepo) List() ([]OrgMailAccount, error) {
	rows, err := r.db.Query(
		`SELECT id, name, host, port, username, password, from_name, from_email, use_tls, is_active, created_at, updated_at
		 FROM org_mail_smtp_accounts ORDER BY name ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("org mail account: list: %w", err)
	}
	defer rows.Close()

	out := make([]OrgMailAccount, 0)
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

func scanAccountRow(r scannableAccount) (*OrgMailAccount, error) {
	var a OrgMailAccount
	err := r.Scan(
		&a.ID, &a.Name, &a.Host, &a.Port, &a.Username, &a.Password,
		&a.FromName, &a.FromEmail, &a.UseTLS, &a.IsActive,
		&a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("org mail account: scan: %w", err)
	}
	return &a, nil
}
