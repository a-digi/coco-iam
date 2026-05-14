// Package producers contains the four event listeners that enqueue
// application-user-cleanup tasks in response to organization/group delete
// events. Each listener implements the full MethodEventListener interface
// (BeforeExecution + AfterExecution). Resolution of user ids happens in
// BeforeExecution (reading the row before delete); publishing happens in
// AfterExecution (after the delete has succeeded).
//
// The manager is resolved from the request's DI ContextBag.
package producers

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/a-digi/coco-iam/src/applications/cleanup"
	"github.com/a-digi/coco-queue"
	"github.com/a-digi/coco-server/server/request"
)

// keyedGetter is the narrow interface this package needs from the DI
// ContextBag. Declaring it locally avoids an import cycle between
// config/di → config/resource → this package → config/di.
type keyedGetter interface {
	Get(key string) (interface{}, bool)
}

// resolveManager extracts the queue manager from the DI context.
func resolveManager(reqCtx request.RequestContext) queue.Manager {
	ctx := reqCtx.GetDI()
	bag, ok := ctx.(keyedGetter)
	if !ok {
		return nil
	}
	raw, ok := bag.Get(queue.ContextBagKey)
	if !ok {
		return nil
	}
	mgr, ok := raw.(queue.Manager)
	if !ok {
		return nil
	}
	return mgr
}

// publish is a small helper that enqueues one cleanup task per user id.
// Errors are logged but never propagated — the delete that triggered this has
// already succeeded.
func publish(reqCtx request.RequestContext, userIDs ...string) {
	mgr := resolveManager(reqCtx)
	if mgr == nil {
		return
	}
	log := reqCtx.GetDI().GetLogger()
	for _, uid := range userIDs {
		if uid == "" {
			continue
		}
		if err := mgr.Publish("application-user-cleanup", cleanup.Payload{UserID: uid}); err != nil {
			log.Warning("application-user-cleanup enqueue failed for user %s: %v", uid, err)
		}
	}
}

// pendingUserIDs is a tiny per-request cache used by (c) and (d) listeners
// that need to resolve multiple users in BeforeExecution and fan out in
// AfterExecution. Keyed by entity pointer identity to keep listener state
// out of global scope.
type pendingUserIDs struct {
	mu    sync.Mutex
	items map[interface{}][]string
}

func (p *pendingUserIDs) put(k interface{}, ids []string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.items == nil {
		p.items = make(map[interface{}][]string)
	}
	p.items[k] = ids
}

func (p *pendingUserIDs) take(k interface{}) []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	ids := p.items[k]
	delete(p.items, k)
	return ids
}

// Helper to pull a named string field out of any entity via JSON round-trip.
// We use this because listeners receive `entity interface{}` — reflecting on
// generic struct layouts via JSON is simple and consistent.
func extractString(entity interface{}, field string) (string, error) {
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
