// Package persistent holds write-only repositories over an
// organization's own users.db mail tables — the write half of
// api/src/organizations/mail/repository/query. Each repo is
// self-contained (no cross-import of the query package) so reads and
// writes stay fully independent; handlers are responsible for loading
// existing state via the query repo before calling Update, and for
// re-reading after a write to build the HTTP response — mirrors the
// loginbans/attackbans settings handlers' own "re-read rather than
// echo back" convention. See plan/org-app-email-settings/plan.md.
package persistent

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
)

var (
	ErrNotFound      = errors.New("org mail account: not found")
	ErrDuplicateName = errors.New("org mail account: name already exists")
	ErrActiveAccount = errors.New("org mail account: cannot delete the active account")
)

// OrgMailAccountWrite is the full row a Create/Update call writes.
type OrgMailAccountWrite struct {
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
}

type OrgMailAccountsPersistentRepo struct {
	db *sql.DB
}

func NewOrgMailAccountsPersistentRepo(db *sql.DB) *OrgMailAccountsPersistentRepo {
	return &OrgMailAccountsPersistentRepo{db: db}
}

// Create inserts a new account, generating an ID if none was supplied.
// If IsActive is set, every other account is demoted first in the same
// transaction — mirrors accounts.Store.Create.
func (r *OrgMailAccountsPersistentRepo) Create(a OrgMailAccountWrite) (string, error) {
	if a.ID == "" {
		a.ID = newUUID()
	}
	tx, err := r.db.Begin()
	if err != nil {
		return "", fmt.Errorf("org mail account: begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	if a.IsActive {
		if _, err := tx.Exec(`UPDATE org_mail_smtp_accounts SET is_active = FALSE, updated_at = CURRENT_TIMESTAMP`); err != nil {
			return "", fmt.Errorf("org mail account: demote existing: %w", err)
		}
	}

	_, err = tx.Exec(
		`INSERT INTO org_mail_smtp_accounts
		 (id, name, host, port, username, password, from_name, from_email, use_tls, is_active, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		a.ID, a.Name, a.Host, a.Port, a.Username, a.Password, a.FromName, a.FromEmail, a.UseTLS, a.IsActive,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return "", ErrDuplicateName
		}
		return "", fmt.Errorf("org mail account: insert: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("org mail account: commit: %w", err)
	}
	return a.ID, nil
}

// Update replaces every mutable field on the row keyed by a.ID. Callers
// are expected to have already merged the patch onto the existing row
// (via the query repo) — this method does a full write, not a partial
// merge, keeping this package independent of the query package.
func (r *OrgMailAccountsPersistentRepo) Update(a OrgMailAccountWrite) error {
	res, err := r.db.Exec(
		`UPDATE org_mail_smtp_accounts
		 SET host = ?, port = ?, username = ?, password = ?, from_name = ?, from_email = ?, use_tls = ?, updated_at = CURRENT_TIMESTAMP
		 WHERE id = ?`,
		a.Host, a.Port, a.Username, a.Password, a.FromName, a.FromEmail, a.UseTLS, a.ID,
	)
	if err != nil {
		return fmt.Errorf("org mail account: update: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete hard-deletes the account. Refuses if it's the active one.
func (r *OrgMailAccountsPersistentRepo) Delete(id string) error {
	var isActive bool
	err := r.db.QueryRow(`SELECT is_active FROM org_mail_smtp_accounts WHERE id = ?`, id).Scan(&isActive)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("org mail account: check active before delete: %w", err)
	}
	if isActive {
		return ErrActiveAccount
	}
	res, err := r.db.Exec(`DELETE FROM org_mail_smtp_accounts WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("org mail account: delete: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// Activate demotes every other account and promotes id — mirrors
// accounts.Store.Activate exactly.
func (r *OrgMailAccountsPersistentRepo) Activate(id string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("org mail account: begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.Exec(`UPDATE org_mail_smtp_accounts SET is_active = FALSE, updated_at = CURRENT_TIMESTAMP WHERE is_active = TRUE`); err != nil {
		return fmt.Errorf("org mail account: demote: %w", err)
	}
	res, err := tx.Exec(`UPDATE org_mail_smtp_accounts SET is_active = TRUE, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("org mail account: promote: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("org mail account: commit: %w", err)
	}
	return nil
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return len(msg) > 0 && stringContains(msg, "UNIQUE constraint failed")
}

func stringContains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func newUUID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "00000000-0000-0000-0000-000000000000"
	}
	hx := hex.EncodeToString(buf)
	return fmt.Sprintf("%s-%s-%s-%s-%s", hx[:8], hx[8:12], hx[12:16], hx[16:20], hx[20:32])
}
