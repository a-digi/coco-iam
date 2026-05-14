package userrules

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/a-digi/coco-orm/orm"
)

// AdminStore persists the single admin-wide rule set in admin_user_rule_sets.
// The row is keyed by the fixed sentinel id = 'admin'.
type AdminStore struct {
	db *sql.DB
}

// NewAdminStore wires an AdminStore to the global users.db.
func NewAdminStore(dbm *orm.DatabaseManager) *AdminStore {
	return &AdminStore{db: dbm.Connector.DB}
}

// Get returns the admin rule set. Returns Defaults() when no row exists.
func (s *AdminStore) Get() (RuleSet, error) {
	var raw string
	err := s.db.QueryRow(
		`SELECT rules_json FROM admin_user_rule_sets WHERE id = 'admin' LIMIT 1`,
	).Scan(&raw)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Defaults(), nil
		}
		return Defaults(), fmt.Errorf("userrules: admin get: %w", err)
	}
	var rs RuleSet
	if err := json.Unmarshal([]byte(raw), &rs); err != nil {
		return Defaults(), fmt.Errorf("userrules: admin decode: %w", err)
	}
	return rs, nil
}

// Upsert writes (or replaces) the admin rule set.
func (s *AdminStore) Upsert(rs RuleSet) error {
	raw, err := json.Marshal(rs)
	if err != nil {
		return fmt.Errorf("userrules: admin encode: %w", err)
	}
	_, err = s.db.Exec(
		`INSERT INTO admin_user_rule_sets (id, rules_json, updated_at)
		 VALUES ('admin', ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(id) DO UPDATE SET
		    rules_json = excluded.rules_json,
		    updated_at = CURRENT_TIMESTAMP`,
		string(raw),
	)
	if err != nil {
		return fmt.Errorf("userrules: admin upsert: %w", err)
	}
	return nil
}
