package settings

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/a-digi/coco-orm/orm"
)

// Store is the KV wrapper over mail.db.mail_settings. Every key is a
// string; typed accessors live in resolver.go so callers don't sprinkle
// strconv.Atoi / strconv.ParseBool everywhere.
type Store struct {
	db *sql.DB
}

// NewStore binds a Store to the mail DatabaseManager.
func NewStore(dbm *orm.DatabaseManager) *Store {
	return &Store{db: dbm.Connector.DB}
}

// NewStoreFromDB binds a Store directly to any *sql.DB — used in tests.
func NewStoreFromDB(db *sql.DB) *Store {
	return &Store{db: db}
}

// Get returns (value, true) if the key exists, otherwise ("", false).
func (s *Store) Get(key string) (string, bool, error) {
	var value string
	err := s.db.QueryRow(`SELECT value FROM mail_settings WHERE key = ?`, key).Scan(&value)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("mail settings: get %q: %w", key, err)
	}
	return value, true, nil
}

// All returns every row in the table. Used by the admin GET endpoint.
func (s *Store) All() (map[string]string, error) {
	rows, err := s.db.Query(`SELECT key, value FROM mail_settings`)
	if err != nil {
		return nil, fmt.Errorf("mail settings: list: %w", err)
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, fmt.Errorf("mail settings: scan: %w", err)
		}
		out[k] = v
	}
	return out, rows.Err()
}

// Set upserts a key. Passing an empty value clears the key but keeps the
// row — useful for "reset to env fallback" semantics from the admin UI.
func (s *Store) Set(key, value string) error {
	_, err := s.db.Exec(
		`INSERT INTO mail_settings (key, value, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP`,
		key, value,
	)
	if err != nil {
		return fmt.Errorf("mail settings: set %q: %w", key, err)
	}
	return nil
}

// SetMany is a small convenience for the PATCH handler.
func (s *Store) SetMany(kv map[string]string) error {
	for k, v := range kv {
		if err := s.Set(k, v); err != nil {
			return err
		}
	}
	return nil
}

// Delete removes a key entirely. Unused on day one but convenient for
// future "reset to defaults" buttons.
func (s *Store) Delete(key string) error {
	_, err := s.db.Exec(`DELETE FROM mail_settings WHERE key = ?`, key)
	if err != nil {
		return fmt.Errorf("mail settings: delete %q: %w", key, err)
	}
	return nil
}

// KeysWithValue returns every key whose value matches, filtered to those
// starting with `prefix`. Used by the accounts delete-guard to find event
// bindings that point at a given account name.
func (s *Store) KeysWithValue(prefix, value string) ([]string, error) {
	rows, err := s.db.Query(
		`SELECT key FROM mail_settings WHERE value = ? AND key LIKE ?`,
		value, prefix+"%",
	)
	if err != nil {
		return nil, fmt.Errorf("mail settings: KeysWithValue: %w", err)
	}
	defer rows.Close()
	out := make([]string, 0)
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, fmt.Errorf("mail settings: scan: %w", err)
		}
		out = append(out, k)
	}
	return out, rows.Err()
}
