// Package listener wires the organization lifecycle to the profile
// subsystem: on create, provision a per-org SQLite database so profile
// fields can be defined and user values stored immediately after.
package listener

import (
	"encoding/json"

	"github.com/a-digi/coco-iam/src/organizations/profile/dbregistry"
	"github.com/a-digi/coco-server/server/request"
)

// ProvisionOrgDBOnCreate runs after an organization row is inserted.
// It resolves the OrgDBRegistry from DI and asks it to Provision the
// new org's database file + apply migrations. Failures are logged but
// do not roll back the organization insert — the DB can be lazily
// created on first profile request as a fallback.
type ProvisionOrgDBOnCreate struct{}

// BeforeExecution is unused.
func (l *ProvisionOrgDBOnCreate) BeforeExecution(_ request.RequestContext, _ interface{}) error {
	return nil
}

// AfterExecution reads the new org id from the persisted entity and
// triggers registry.Provision.
func (l *ProvisionOrgDBOnCreate) AfterExecution(reqCtx request.RequestContext, entity interface{}) {
	log := reqCtx.GetDI().GetLogger()
	if entity == nil {
		return
	}
	id, err := extractOrgID(entity)
	if err != nil || id == "" {
		if log != nil {
			log.Warning("org profile db: could not read org id from created entity: %v", err)
		}
		return
	}

	registry := resolveRegistry(reqCtx)
	if registry == nil {
		if log != nil {
			log.Warning("org profile db: registry not available in DI; skipping provision for %s", id)
		}
		return
	}
	if err := registry.Provision(id); err != nil {
		if log != nil {
			log.Warning("org profile db: provision %s: %v", id, err)
		}
	}
}

type bagGetter interface {
	Get(key string) (interface{}, bool)
}

func resolveRegistry(reqCtx request.RequestContext) *dbregistry.OrgDBRegistry {
	bag, ok := reqCtx.GetDI().(bagGetter)
	if !ok {
		return nil
	}
	raw, ok := bag.Get(dbregistry.ContextBagKey)
	if !ok {
		return nil
	}
	reg, _ := raw.(*dbregistry.OrgDBRegistry)
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
