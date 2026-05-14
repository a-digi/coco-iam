// Package listener wires the organization lifecycle to the per-org
// OAuth database: on create, provision the SQLite file so token rows
// can be written immediately when the first OAuth flow runs for an
// application in this org.
//
// Mirror of organizations/users/listener and
// applications/apicredentials/listener — registered in parallel on
// the `organizations` resource so every new org gets all files.
package listener

import (
	"encoding/json"

	"github.com/a-digi/coco-iam/src/oauthserver/dbregistry"
	"github.com/a-digi/coco-server/server/request"
)

// ProvisionOAuthDBOnCreate runs after an organization row is inserted.
// Failures are logged but do not roll back the insert — the DB can be
// lazily created on the first OAuth request as a fallback.
type ProvisionOAuthDBOnCreate struct{}

// BeforeExecution is unused.
func (l *ProvisionOAuthDBOnCreate) BeforeExecution(_ request.RequestContext, _ interface{}) error {
	return nil
}

// AfterExecution reads the new org id from the persisted entity and
// triggers registry.Provision.
func (l *ProvisionOAuthDBOnCreate) AfterExecution(reqCtx request.RequestContext, entity interface{}) {
	log := reqCtx.GetDI().GetLogger()
	if entity == nil {
		return
	}
	id, err := extractOrgID(entity)
	if err != nil || id == "" {
		if log != nil {
			log.Warning("org oauth db: could not read org id from created entity: %v", err)
		}
		return
	}

	registry := resolveRegistry(reqCtx)
	if registry == nil {
		if log != nil {
			log.Warning("org oauth db: registry not available in DI; skipping provision for %s", id)
		}
		return
	}
	if err := registry.Provision(id); err != nil {
		if log != nil {
			log.Warning("org oauth db: provision %s: %v", id, err)
		}
	}
}

type bagGetter interface {
	Get(key string) (interface{}, bool)
}

func resolveRegistry(reqCtx request.RequestContext) *dbregistry.OrgOAuthDBRegistry {
	bag, ok := reqCtx.GetDI().(bagGetter)
	if !ok {
		return nil
	}
	raw, ok := bag.Get(dbregistry.ContextBagKey)
	if !ok {
		return nil
	}
	reg, _ := raw.(*dbregistry.OrgOAuthDBRegistry)
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
