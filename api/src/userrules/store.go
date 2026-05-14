package userrules

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/a-digi/coco-iam/src/organizations/users/dbregistry"
	"github.com/a-digi/coco-iam/src/orgrouter"
	"github.com/a-digi/coco-orm/orm"
)

// Store is a façade over AdminStore and OrgStore. All existing callers use
// this type; the sub-stores are not exposed outside the package.
type Store struct {
	db          *sql.DB                        // admin user lookup in GetForUser
	orgRegistry *dbregistry.OrgUserDBRegistry  // org routing in GetForUser
	adminStore  *AdminStore
	orgStore    *OrgStore
}

// NewStore binds a Store to the main users.db manager and the per-org registry.
func NewStore(dbm *orm.DatabaseManager, orgRegistry *dbregistry.OrgUserDBRegistry) *Store {
	return &Store{
		db:          dbm.Connector.DB,
		orgRegistry: orgRegistry,
		adminStore:  NewAdminStore(dbm),
		orgStore:    NewOrgStore(orgRegistry),
	}
}

// GetAdmin returns the admin-wide rule set. Missing row → defaults.
func (s *Store) GetAdmin() (RuleSet, error) {
	return s.adminStore.Get()
}

// GetForOrg returns an organization's rule set. Missing row → defaults.
func (s *Store) GetForOrg(orgID string) (RuleSet, error) {
	return s.orgStore.GetForOrg(orgID)
}

// GetForUser resolves which rule set applies to `userID`:
//   - If the ID belongs to an admin user, admin rules apply.
//   - Otherwise the user's organization rules apply.
//   - If the user can't be located at all, defaults are returned.
func (s *Store) GetForUser(userID string) (RuleSet, error) {
	if userID == "" {
		return Defaults(), nil
	}
	// Admin first — the table is small and the check is cheap.
	var adminExists int
	if err := s.db.QueryRow(
		`SELECT 1 FROM admin_users WHERE id = ? LIMIT 1`, userID,
	).Scan(&adminExists); err == nil {
		return s.GetAdmin()
	} else if !errors.Is(err, sql.ErrNoRows) {
		return Defaults(), fmt.Errorf("userrules: resolve admin user: %w", err)
	}

	if s.orgRegistry == nil {
		return Defaults(), nil
	}
	_, orgID, err := orgrouter.OrgDBFor(s.orgRegistry, userID)
	if err != nil {
		return Defaults(), nil
	}
	return s.GetForOrg(orgID)
}

// UpsertAdmin writes (or replaces) the admin rule set.
func (s *Store) UpsertAdmin(rs RuleSet) error {
	return s.adminStore.Upsert(rs)
}

// UpsertForOrg writes (or replaces) an organization's rule set.
func (s *Store) UpsertForOrg(orgID string, rs RuleSet) error {
	return s.orgStore.UpsertForOrg(orgID, rs)
}
