package userrules

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/a-digi/coco-iam/src/organizations/users/dbregistry"
	"github.com/a-digi/coco-iam/src/orgrouter"
)

// OrgStore persists per-organization rule sets in each org's user_rule_sets
// table. The row is keyed by the fixed sentinel id = 'default'.
type OrgStore struct {
	openDB func(orgID string) (*sql.DB, error)
}

// NewOrgStore wires an OrgStore to the per-org registry.
func NewOrgStore(reg *dbregistry.OrgUserDBRegistry) *OrgStore {
	return &OrgStore{
		openDB: func(orgID string) (*sql.DB, error) { return orgrouter.ForOrg(reg, orgID) },
	}
}

// NewOrgStoreFromFunc wires an OrgStore to an arbitrary openDB function.
// Intended for tests and DI scenarios that supply a DB without a registry.
func NewOrgStoreFromFunc(openDB func(orgID string) (*sql.DB, error)) *OrgStore {
	return &OrgStore{openDB: openDB}
}

// GetForOrg returns an org's rule set. Returns Defaults() when the row is
// absent or the org DB cannot be resolved.
func (s *OrgStore) GetForOrg(orgID string) (RuleSet, error) {
	if orgID == "" {
		return Defaults(), nil
	}
	orgDB, err := s.openDB(orgID)
	if err != nil || orgDB == nil {
		return Defaults(), nil
	}
	var raw string
	err = orgDB.QueryRow(
		`SELECT rules_json FROM user_rule_sets WHERE id = 'default' LIMIT 1`,
	).Scan(&raw)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Defaults(), nil
		}
		return Defaults(), fmt.Errorf("userrules: org get %s: %w", orgID, err)
	}
	var rs RuleSet
	if err := json.Unmarshal([]byte(raw), &rs); err != nil {
		return Defaults(), fmt.Errorf("userrules: org decode %s: %w", orgID, err)
	}
	return rs, nil
}

// UpsertForOrg writes (or replaces) an org's rule set.
func (s *OrgStore) UpsertForOrg(orgID string, rs RuleSet) error {
	if orgID == "" {
		return errors.New("userrules: orgID required")
	}
	orgDB, err := s.openDB(orgID)
	if err != nil {
		return fmt.Errorf("userrules: org open db %s: %w", orgID, err)
	}
	raw, err := json.Marshal(rs)
	if err != nil {
		return fmt.Errorf("userrules: org encode: %w", err)
	}
	_, err = orgDB.Exec(
		`INSERT INTO user_rule_sets (id, rules_json, updated_at)
		 VALUES ('default', ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(id) DO UPDATE SET
		    rules_json = excluded.rules_json,
		    updated_at = CURRENT_TIMESTAMP`,
		string(raw),
	)
	if err != nil {
		return fmt.Errorf("userrules: org upsert %s: %w", orgID, err)
	}
	return nil
}
