// Package listener wires the organization lifecycle to the per-org
// user-database subsystem: on create, provision a user SQLite file so
// end-user rows (users, passwords, ACLs, group memberships) can be
// written immediately after.
//
// Mirror of organizations/profile/listener — registered in parallel on
// the `organizations` resource so every new org gets both files.
package listener

import (
	"encoding/json"

	"github.com/a-digi/coco-iam/src/organizations/users/dbregistry"
	"github.com/a-digi/coco-server/server/request"
)

// ProvisionUsersDBOnCreate runs after an organization row is
// inserted. It resolves the OrgUserDBRegistry from DI and asks it to
// Provision the new org's user database file + apply migrations.
// Failures are logged but do not roll back the organization insert —
// the DB can be lazily created on the first user-touching request as
// a fallback.
type ProvisionUsersDBOnCreate struct{}

// BeforeExecution is unused.
func (l *ProvisionUsersDBOnCreate) BeforeExecution(_ request.RequestContext, _ interface{}) error {
	return nil
}

// AfterExecution reads the new org id from the persisted entity and
// triggers registry.Provision.
func (l *ProvisionUsersDBOnCreate) AfterExecution(reqCtx request.RequestContext, entity interface{}) {
	log := reqCtx.GetDI().GetLogger()
	if entity == nil {
		return
	}
	id, err := extractOrgID(entity)
	if err != nil || id == "" {
		if log != nil {
			log.Warning("org users db: could not read org id from created entity: %v", err)
		}
		return
	}

	registry := resolveRegistry(reqCtx)
	if registry == nil {
		if log != nil {
			log.Warning("org users db: registry not available in DI; skipping provision for %s", id)
		}
		return
	}
	if err := registry.Provision(id); err != nil {
		if log != nil {
			log.Warning("org users db: provision %s: %v", id, err)
		}
	}
}

type bagGetter interface {
	Get(key string) (interface{}, bool)
}

func resolveRegistry(reqCtx request.RequestContext) *dbregistry.OrgUserDBRegistry {
	bag, ok := reqCtx.GetDI().(bagGetter)
	if !ok {
		return nil
	}
	raw, ok := bag.Get(dbregistry.ContextBagKey)
	if !ok {
		return nil
	}
	reg, _ := raw.(*dbregistry.OrgUserDBRegistry)
	return reg
}

func extractOrgID(entity interface{}) (string, error) {
	raw, err := json.Marshal(entity)
	if err != nil {
		return "", err
	}
	var peek struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &peek); err != nil {
		return "", err
	}
	return peek.ID, nil
}
