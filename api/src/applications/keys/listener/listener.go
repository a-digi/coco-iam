// Package listener owns the PostEventListener that the `applications`
// resource registration hooks into. On application create we:
//
//  1. Ensure an RSA signing keypair is on disk (so the app can mint
//     tokens on day one).
//  2. Seed the twelve default `users:` / `groups:` / `acl:` /
//     `scopes:` scope rows into `application_scopes` so admins can
//     assign them from the existing ACL form without any manual
//     pre-setup step.
package listener

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/a-digi/coco-iam/src/applications/keys"
	"github.com/a-digi/coco-iam/src/organizations/users/dbregistry"
	"github.com/a-digi/coco-iam/src/orgrouter"
	"github.com/a-digi/coco-server/server/request"
)

// defaultApplicationScopes is the scope vocabulary every new
// application auto-receives. Admins can remove any they don't want
// exposed for a given application.
var defaultApplicationScopes = []struct {
	ID          string
	Description string
}{
	{"users:read", "Read users in this application."},
	{"users:write", "Create and modify users in this application."},
	{"users:delete", "Soft-delete users in this application."},
	{"groups:read", "Read user groups mapped to this application."},
	{"groups:write", "Create and modify user groups in this application."},
	{"groups:delete", "Soft-delete user groups in this application."},
	{"acl:read", "Read ACL assignments for users and groups."},
	{"acl:write", "Create and modify ACL assignments."},
	{"acl:delete", "Soft-delete ACL assignments."},
	{"scopes:read", "Read this application's scope catalog."},
	{"scopes:write", "Create and modify this application's scope catalog."},
	{"scopes:delete", "Soft-delete scopes from this application's catalog."},
}

// EnsureOnCreate resolves the keys service from DI at invocation time
// — that way the listener has no init-order dependency on the service
// wiring in main.go. The struct itself is stateless, so a single
// package-level instance is safe to reuse.
type EnsureOnCreate struct{}

// BeforeExecution is unused: we need the inserted row's ID which only
// exists after the row is persisted.
func (l *EnsureOnCreate) BeforeExecution(_ request.RequestContext, _ interface{}) error {
	return nil
}

// AfterExecution reads the application id off the created entity and
// asks the keys service to generate (or verify) its keypair. Failures
// are logged but not propagated — a successful application insert
// should not be reverted just because the filesystem is temporarily
// unhappy; the handler can regenerate later.
func (l *EnsureOnCreate) AfterExecution(reqCtx request.RequestContext, entity interface{}) {
	log := reqCtx.GetDI().GetLogger()
	if entity == nil {
		return
	}
	id, err := extractAppID(entity)
	if err != nil || id == "" {
		if log != nil {
			log.Warning("app keys: could not read application id from created entity: %v", err)
		}
		return
	}
	svc := resolveService(reqCtx)
	if svc == nil {
		if log != nil {
			log.Warning("app keys: service not available in DI; skipping keypair generation for %s", id)
		}
		return
	}
	if err := svc.EnsureActive(id); err != nil {
		if log != nil {
			log.Warning("app keys: generate keypair for %s: %v", id, err)
		}
	}

	// Seed the default scope vocabulary. Best-effort — we don't
	// block the application create on failure here. A duplicate
	// (admin created the app twice, or seeded scopes manually) is
	// tolerated via INSERT OR IGNORE.
	seedDefaultScopes(reqCtx, id)
}

func seedDefaultScopes(reqCtx request.RequestContext, appID string) {
	log := reqCtx.GetDI().GetLogger()

	diCtx := reqCtx.GetDI()
	bag, ok := diCtx.(bagGetter)
	if !ok {
		if log != nil {
			log.Warning("app scopes: DI context not keyed")
		}
		return
	}
	raw, ok := bag.Get(dbregistry.ContextBagKey)
	if !ok {
		if log != nil {
			log.Warning("app scopes: org user db registry not in DI")
		}
		return
	}
	reg, ok := raw.(*dbregistry.OrgUserDBRegistry)
	if !ok {
		if log != nil {
			log.Warning("app scopes: org user db registry type mismatch")
		}
		return
	}
	orgDB, _, err := orgrouter.OrgDBForApp(reg, appID)
	if err != nil {
		if log != nil {
			log.Warning("app scopes: resolve org for %s: %v", appID, err)
		}
		return
	}

	for _, scope := range defaultApplicationScopes {
		id, err := newScopeID()
		if err != nil {
			if log != nil {
				log.Warning("app scopes: newID: %v", err)
			}
			continue
		}
		_, err = orgDB.Exec(
			`INSERT OR IGNORE INTO application_scopes
			   (id, application_id, scope_id, description, created_at, is_active)
			 VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP, TRUE)`,
			id, appID, scope.ID, scope.Description,
		)
		if err != nil && log != nil {
			log.Warning("app scopes: seed %s/%s: %v", appID, scope.ID, err)
		}
	}
}

func newScopeID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	hx := hex.EncodeToString(buf)
	return fmt.Sprintf("%s-%s-%s-%s-%s", hx[:8], hx[8:12], hx[12:16], hx[16:20], hx[20:32]), nil
}

type bagGetter interface {
	Get(key string) (interface{}, bool)
}

// resolveService reaches the keys service out of the DI bag. Nil is a
// valid signal that the service hasn't been registered yet — the
// caller logs and skips.
func resolveService(reqCtx request.RequestContext) *keys.Service {
	ctx := reqCtx.GetDI()
	bag, ok := ctx.(bagGetter)
	if !ok {
		return nil
	}
	raw, ok := bag.Get(keys.ContextBagKeyService)
	if !ok {
		return nil
	}
	svc, _ := raw.(*keys.Service)
	return svc
}

// extractAppID pulls `.id` out of the entity regardless of whether
// the upstream gave us an `*Application` struct or a generic map.
// JSON round-trip is the simplest path that works for both.
func extractAppID(entity interface{}) (string, error) {
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
