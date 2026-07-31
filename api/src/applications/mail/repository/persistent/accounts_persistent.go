// Package persistent holds write-only repositories over an
// application's own mail tables — the write half of
// api/src/applications/mail/repository/query. Each repo is
// self-contained (no cross-import of the query package) so reads and
// writes stay fully independent; handlers are responsible for loading
// existing state via the query repo before calling Update, and for
// re-reading after a write to build the HTTP response — mirrors the
// org tier's own "re-read rather than echo back" convention. See
// plan/org-app-email-settings/plan.md.
package persistent

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
)

var (
	ErrNotFound      = errors.New("app mail account: not found")
	ErrDuplicateName = errors.New("app mail account: name already exists")
	ErrActiveAccount = errors.New("app mail account: cannot delete the active account")
)

// AppMailAccountWrite is the full row a Create/Update call writes.
type AppMailAccountWrite struct {
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

type AppMailAccountsPersistentRepo struct {
	db    *sql.DB
	appID string
}

func NewAppMailAccountsPersistentRepo(db *sql.DB, appID string) *AppMailAccountsPersistentRepo {
	return &AppMailAccountsPersistentRepo{db: db, appID: appID}
}

// Create inserts a new account, generating an ID if none was supplied.
// If IsActive is set, every other account of this application is
// demoted first in the same transaction — mirrors the org tier's
// Create.
func (r *AppMailAccountsPersistentRepo) Create(a AppMailAccountWrite) (string, error) {
	if a.ID == "" {
		a.ID = newUUID()
	}
	tx, err := r.db.Begin()
	if err != nil {
		return "", fmt.Errorf("app mail account: begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	if a.IsActive {
		if _, err := tx.Exec(
			`UPDATE app_mail_smtp_accounts SET is_active = FALSE, updated_at = CURRENT_TIMESTAMP WHERE application_id = ?`,
			r.appID,
		); err != nil {
			return "", fmt.Errorf("app mail account: demote existing: %w", err)
		}
	}

	_, err = tx.Exec(
		`INSERT INTO app_mail_smtp_accounts
		 (id, application_id, name, host, port, username, password, from_name, from_email, use_tls, is_active, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		a.ID, r.appID, a.Name, a.Host, a.Port, a.Username, a.Password, a.FromName, a.FromEmail, a.UseTLS, a.IsActive,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return "", ErrDuplicateName
		}
		return "", fmt.Errorf("app mail account: insert: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("app mail account: commit: %w", err)
	}
	return a.ID, nil
}

// Update replaces every mutable field on the row keyed by a.ID (scoped
// to this application). Callers are expected to have already merged
// the patch onto the existing row (via the query repo) — this method
// does a full write, not a partial merge.
func (r *AppMailAccountsPersistentRepo) Update(a AppMailAccountWrite) error {
	res, err := r.db.Exec(
		`UPDATE app_mail_smtp_accounts
		 SET host = ?, port = ?, username = ?, password = ?, from_name = ?, from_email = ?, use_tls = ?, updated_at = CURRENT_TIMESTAMP
		 WHERE id = ? AND application_id = ?`,
		a.Host, a.Port, a.Username, a.Password, a.FromName, a.FromEmail, a.UseTLS, a.ID, r.appID,
	)
	if err != nil {
		return fmt.Errorf("app mail account: update: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete hard-deletes the account. Refuses if it's the active one.
func (r *AppMailAccountsPersistentRepo) Delete(id string) error {
	var isActive bool
	err := r.db.QueryRow(
		`SELECT is_active FROM app_mail_smtp_accounts WHERE id = ? AND application_id = ?`, id, r.appID,
	).Scan(&isActive)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("app mail account: check active before delete: %w", err)
	}
	if isActive {
		return ErrActiveAccount
	}
	res, err := r.db.Exec(`DELETE FROM app_mail_smtp_accounts WHERE id = ? AND application_id = ?`, id, r.appID)
	if err != nil {
		return fmt.Errorf("app mail account: delete: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// Activate demotes every other account of this application and
// promotes id — mirrors the org tier's Activate exactly, one level
// deeper (scoped by application_id).
func (r *AppMailAccountsPersistentRepo) Activate(id string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("app mail account: begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.Exec(
		`UPDATE app_mail_smtp_accounts SET is_active = FALSE, updated_at = CURRENT_TIMESTAMP WHERE application_id = ? AND is_active = TRUE`,
		r.appID,
	); err != nil {
		return fmt.Errorf("app mail account: demote: %w", err)
	}
	res, err := tx.Exec(
		`UPDATE app_mail_smtp_accounts SET is_active = TRUE, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND application_id = ?`,
		id, r.appID,
	)
	if err != nil {
		return fmt.Errorf("app mail account: promote: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("app mail account: commit: %w", err)
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
