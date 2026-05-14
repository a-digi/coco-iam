package accounts

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/a-digi/coco-orm/orm"
)

// Store is the CRUD + activation layer over mail.db.mail_smtp_accounts.
type Store struct {
	db *sql.DB
}

// NewStore binds a Store to the mail DatabaseManager.
func NewStore(dbm *orm.DatabaseManager) *Store {
	return &Store{db: dbm.Connector.DB}
}

// Sentinel errors returned by the store.
var (
	ErrNotFound       = errors.New("mail account: not found")
	ErrDuplicateName  = errors.New("mail account: name already exists")
	ErrActiveAccount  = errors.New("mail account: cannot delete the active account")
	ErrNoActive       = errors.New("mail account: no active account")
)

// Create inserts a new account. If IsActive is set, existing actives are
// demoted inside the same transaction so the partial unique index stays
// happy.
func (s *Store) Create(a Account) (Account, error) {
	if a.ID == "" {
		a.ID = newUUID()
	}
	tx, err := s.db.Begin()
	if err != nil {
		return Account{}, fmt.Errorf("mail account: begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	if a.IsActive {
		if _, err := tx.Exec(`UPDATE mail_smtp_accounts SET is_active = FALSE, updated_at = CURRENT_TIMESTAMP`); err != nil {
			return Account{}, fmt.Errorf("mail account: demote existing: %w", err)
		}
	}

	_, err = tx.Exec(
		`INSERT INTO mail_smtp_accounts
		 (id, name, host, port, username, password, from_name, from_email, use_tls, is_active, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		a.ID, a.Name, a.Host, a.Port, a.Username, a.Password, a.FromName, a.FromEmail, a.UseTLS, a.IsActive,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return Account{}, ErrDuplicateName
		}
		return Account{}, fmt.Errorf("mail account: insert: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Account{}, fmt.Errorf("mail account: commit: %w", err)
	}
	created, gerr := s.Get(a.ID)
	if gerr != nil {
		return Account{}, gerr
	}
	return *created, nil
}

// Get returns a single account by id.
func (s *Store) Get(id string) (*Account, error) {
	row := s.db.QueryRow(
		`SELECT id, name, host, port, username, password, from_name, from_email, use_tls, is_active, created_at, updated_at
		 FROM mail_smtp_accounts WHERE id = ?`, id,
	)
	return scanRow(row)
}

// GetByName resolves an account by its unique name. Used by event
// bindings — events reference accounts by name so a reseed with the same
// name keeps the binding working.
func (s *Store) GetByName(name string) (*Account, error) {
	row := s.db.QueryRow(
		`SELECT id, name, host, port, username, password, from_name, from_email, use_tls, is_active, created_at, updated_at
		 FROM mail_smtp_accounts WHERE name = ?`, name,
	)
	return scanRow(row)
}

// Exists returns true if an account with the given name is present.
func (s *Store) Exists(name string) (bool, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(1) FROM mail_smtp_accounts WHERE name = ?`, name).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("mail account: exists %q: %w", name, err)
	}
	return n > 0, nil
}

// GetActive returns the currently active account or ErrNoActive if none is
// set.
func (s *Store) GetActive() (*Account, error) {
	row := s.db.QueryRow(
		`SELECT id, name, host, port, username, password, from_name, from_email, use_tls, is_active, created_at, updated_at
		 FROM mail_smtp_accounts WHERE is_active = TRUE LIMIT 1`,
	)
	out, err := scanRow(row)
	if err != nil && errors.Is(err, ErrNotFound) {
		return nil, ErrNoActive
	}
	return out, err
}

// List returns every account ordered by name. Pagination isn't needed at
// this scale — accounts are a handful of rows max.
func (s *Store) List() ([]Account, error) {
	rows, err := s.db.Query(
		`SELECT id, name, host, port, username, password, from_name, from_email, use_tls, is_active, created_at, updated_at
		 FROM mail_smtp_accounts ORDER BY name ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("mail account: list: %w", err)
	}
	defer rows.Close()

	out := make([]Account, 0)
	for rows.Next() {
		a, serr := scanRow(rows)
		if serr != nil {
			return nil, serr
		}
		out = append(out, *a)
	}
	return out, rows.Err()
}

// Count returns the number of accounts — used by the migration shim.
func (s *Store) Count() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(1) FROM mail_smtp_accounts`).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("mail account: count: %w", err)
	}
	return n, nil
}

// Update applies a Patch. Empty password preserves the stored value so
// the admin UI doesn't have to re-send the secret on every edit.
func (s *Store) Update(id string, p Patch) (Account, error) {
	existing, err := s.Get(id)
	if err != nil {
		return Account{}, err
	}
	if p.Host != nil {
		existing.Host = *p.Host
	}
	if p.Port != nil {
		existing.Port = *p.Port
	}
	if p.Username != nil {
		existing.Username = *p.Username
	}
	if p.Password != nil && *p.Password != "" {
		existing.Password = *p.Password
	}
	if p.FromName != nil {
		existing.FromName = *p.FromName
	}
	if p.FromEmail != nil {
		existing.FromEmail = *p.FromEmail
	}
	if p.UseTLS != nil {
		existing.UseTLS = *p.UseTLS
	}
	_, err = s.db.Exec(
		`UPDATE mail_smtp_accounts
		 SET host = ?, port = ?, username = ?, password = ?, from_name = ?, from_email = ?, use_tls = ?, updated_at = CURRENT_TIMESTAMP
		 WHERE id = ?`,
		existing.Host, existing.Port, existing.Username, existing.Password,
		existing.FromName, existing.FromEmail, existing.UseTLS, id,
	)
	if err != nil {
		return Account{}, fmt.Errorf("mail account: update: %w", err)
	}
	after, gerr := s.Get(id)
	if gerr != nil {
		return Account{}, gerr
	}
	return *after, nil
}

// Delete hard-deletes the account. Refuses if it's the active one — the
// admin must activate another first.
func (s *Store) Delete(id string) error {
	acc, err := s.Get(id)
	if err != nil {
		return err
	}
	if acc.IsActive {
		return ErrActiveAccount
	}
	res, err := s.db.Exec(`DELETE FROM mail_smtp_accounts WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("mail account: delete: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// Activate demotes every other account and promotes `id`. Wrapped in a
// transaction so the partial unique index never sees a split state.
func (s *Store) Activate(id string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("mail account: begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.Exec(`UPDATE mail_smtp_accounts SET is_active = FALSE, updated_at = CURRENT_TIMESTAMP WHERE is_active = TRUE`); err != nil {
		return fmt.Errorf("mail account: demote: %w", err)
	}
	res, err := tx.Exec(
		`UPDATE mail_smtp_accounts SET is_active = TRUE, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, id,
	)
	if err != nil {
		return fmt.Errorf("mail account: promote: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("mail account: commit: %w", err)
	}
	return nil
}

type scannable interface {
	Scan(dest ...interface{}) error
}

func scanRow(r scannable) (*Account, error) {
	var a Account
	err := r.Scan(
		&a.ID, &a.Name, &a.Host, &a.Port, &a.Username, &a.Password,
		&a.FromName, &a.FromEmail, &a.UseTLS, &a.IsActive,
		&a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("mail account: scan: %w", err)
	}
	return &a, nil
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
