package deleted

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/a-digi/coco-iam/src/organizations/users/dbregistry"
	"github.com/a-digi/coco-iam/src/orgrouter"
	"github.com/a-digi/coco-queue"
	"github.com/a-digi/coco-server/server/request"
)

// OrganizationDeleteListener is registered on the organizations
// resource in config/resource/entities_api_resources.go. It captures a
// snapshot of the org + its workspaces + apps in BeforeExecution
// (while the rows still exist) and publishes the snapshot to
// QueueOrganizationDeleted in AfterExecution (after the delete has
// succeeded).
type OrganizationDeleteListener struct{}

// BeforeExecution reads the org id from the entity, builds the
// snapshot against the main DB, and stashes it keyed by entity
// pointer so AfterExecution can recover it.
func (l *OrganizationDeleteListener) BeforeExecution(reqCtx request.RequestContext, entity interface{}) error {
	log := reqCtx.GetDI().GetLogger()

	orgID, err := extractStringField(entity, "id")
	if err != nil {
		if log != nil {
			log.Warning("organization-deleted: could not read org id from entity: %v", err)
		}
		// Non-fatal: we don't want to block the delete itself. The
		// consumer will treat an empty org id as a no-op.
		return nil
	}

	dbm := reqCtx.GetDI().GetDatabaseManager()
	if dbm == nil || dbm.Connector == nil || dbm.Connector.DB == nil {
		if log != nil {
			log.Warning("organization-deleted: DB manager unavailable, skipping snapshot for %s", orgID)
		}
		return nil
	}

	// Resolve per-org DB for workspace + application snapshot.
	var orgDB *sql.DB
	if bag, ok := reqCtx.GetDI().(keyedGetter); ok {
		if raw, ok := bag.Get(dbregistry.ContextBagKey); ok {
			if reg, ok := raw.(*dbregistry.OrgUserDBRegistry); ok {
				if odb, err := orgrouter.ForOrg(reg, orgID); err == nil {
					orgDB = odb
				}
			}
		}
	}

	snap, err := BuildSnapshot(dbm.Connector.DB, orgDB, orgID)
	if err != nil {
		if log != nil {
			log.Warning("organization-deleted: snapshot %s: %v", orgID, err)
		}
		// Deliberately non-fatal: the delete should still proceed.
		// AfterExecution will publish whatever snapshot we managed to
		// build; an empty snapshot is still useful to the audit row.
	}

	pending.put(entity, Payload{OrganizationID: orgID, Snapshot: snap})
	return nil
}

// AfterExecution publishes the stashed payload. If there's nothing
// stashed (BeforeExecution bailed early) we still publish an empty
// payload — the consumer handles missing org id as a no-op.
func (l *OrganizationDeleteListener) AfterExecution(reqCtx request.RequestContext, entity interface{}) {
	log := reqCtx.GetDI().GetLogger()
	payload := pending.take(entity)
	if payload.OrganizationID == "" {
		return
	}

	mgr := resolveManager(reqCtx)
	if mgr == nil {
		if log != nil {
			log.Warning("organization-deleted: queue manager not in DI; snapshot for %s will not be processed", payload.OrganizationID)
		}
		return
	}

	if err := mgr.Publish("organization-deleted", payload); err != nil {
		if log != nil {
			log.Warning("organization-deleted: enqueue failed for %s: %v", payload.OrganizationID, err)
		}
	}
}

// --- DI helpers -------------------------------------------------------

type keyedGetter interface {
	Get(key string) (interface{}, bool)
}

func resolveManager(reqCtx request.RequestContext) queue.Manager {
	bag, ok := reqCtx.GetDI().(keyedGetter)
	if !ok {
		return nil
	}
	raw, ok := bag.Get(queue.ContextBagKey)
	if !ok {
		return nil
	}
	mgr, _ := raw.(queue.Manager)
	return mgr
}

// --- payload stash ----------------------------------------------------

// pending carries the BeforeExecution snapshot through to
// AfterExecution, keyed by entity pointer identity so listener state
// never leaks between requests.
type pendingStash struct {
	mu    sync.Mutex
	items map[interface{}]Payload
}

func (p *pendingStash) put(k interface{}, v Payload) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.items == nil {
		p.items = make(map[interface{}]Payload)
	}
	p.items[k] = v
}

func (p *pendingStash) take(k interface{}) Payload {
	p.mu.Lock()
	defer p.mu.Unlock()
	v := p.items[k]
	delete(p.items, k)
	return v
}

var pending = &pendingStash{}

// --- entity field reader ----------------------------------------------

func extractStringField(entity interface{}, field string) (string, error) {
	bytes, err := json.Marshal(entity)
	if err != nil {
		return "", err
	}
	var peek map[string]interface{}
	if err := json.Unmarshal(bytes, &peek); err != nil {
		return "", err
	}
	v, ok := peek[field]
	if !ok {
		return "", fmt.Errorf("entity missing field %q", field)
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("entity field %q is not a string", field)
	}
	return s, nil
}
