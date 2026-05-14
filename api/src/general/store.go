package general

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

)

// Store is the KV wrapper over users.db.app_settings.
type Store struct {
	db *sql.DB
}

// NewStoreFromDB binds a Store to any *sql.DB — used for per-org
// settings where the caller already has the org DB handle.
func NewStoreFromDB(db *sql.DB) *Store {
	return &Store{db: db}
}

// Get returns (value, true) if the key exists, otherwise ("", false).
func (s *Store) Get(key string) (string, bool, error) {
	var value string
	err := s.db.QueryRow(`SELECT value FROM app_settings WHERE key = ?`, key).Scan(&value)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("app settings: get %q: %w", key, err)
	}
	return value, true, nil
}

// Set upserts a key.
func (s *Store) Set(key, value string) error {
	_, err := s.db.Exec(
		`INSERT INTO app_settings (id, key, value, updated_at) VALUES (?, ?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP`,
		newUUID(), key, value,
	)
	if err != nil {
		return fmt.Errorf("app settings: set %q: %w", key, err)
	}
	return nil
}

// SetMany writes several keys. Partial failures leave earlier writes in
// place — callers that care should wrap in their own transaction.
func (s *Store) SetMany(kv map[string]string) error {
	for k, v := range kv {
		if err := s.Set(k, v); err != nil {
			return err
		}
	}
	return nil
}

// Delete removes a key entirely.
func (s *Store) Delete(key string) error {
	_, err := s.db.Exec(`DELETE FROM app_settings WHERE key = ?`, key)
	if err != nil {
		return fmt.Errorf("app settings: delete %q: %w", key, err)
	}
	return nil
}

// Snapshot returns the four general.* fields in one call. Empty strings
// indicate the key was never set.
func (s *Store) Snapshot() (Settings, error) {
	out := Settings{}
	for _, pair := range [...][2]*string{
		{stringPtr(KeyBaseURL), &out.BaseURL},
		{stringPtr(KeyPageTitle), &out.PageTitle},
		{stringPtr(KeyDescription), &out.Description},
		{stringPtr(KeyRobots), &out.Robots},
	} {
		v, _, err := s.Get(*pair[0])
		if err != nil {
			return Settings{}, err
		}
		*pair[1] = v
	}
	return out, nil
}

// BaseURL returns the configured frontend URL with any trailing slash
// stripped — convenient for link-builders. Empty when not configured.
func (s *Store) BaseURL() string {
	v, _, _ := s.Get(KeyBaseURL)
	return strings.TrimRight(strings.TrimSpace(v), "/")
}

// PageTitle returns the configured product / instance name. Empty when
// not configured — callers should fall back to something sensible.
func (s *Store) PageTitle() string {
	v, _, _ := s.Get(KeyPageTitle)
	return strings.TrimSpace(v)
}

func newUUID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "00000000-0000-0000-0000-000000000000"
	}
	hx := hex.EncodeToString(buf)
	return fmt.Sprintf("%s-%s-%s-%s-%s", hx[:8], hx[8:12], hx[12:16], hx[16:20], hx[20:32])
}

func stringPtr(s string) *string { return &s }
