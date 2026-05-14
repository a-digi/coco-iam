// Package admin hosts the admin-session HTTP handlers for the
// workspace-application OAuth provider CRUD surface.
// Mounted under /api/v1/applications/{id}/oauth-providers in
// route-application-login.yaml.
package admin

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"

	"github.com/a-digi/coco-lift/resource/uri"

	"github.com/a-digi/coco-iam/src/applications/oauthproviders/repository"
	"github.com/a-digi/coco-iam/src/organizations/users/dbregistry"
	"github.com/a-digi/coco-iam/src/orgrouter"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// appIDFromPath pulls the `{id:<uuid>}` segment out of the URL.
// Matches the pattern used by the sibling admin handlers.
func appIDFromPath(reqCtx request.RequestContext) string {
	key, value := uri.ExtractKeyAndValueFromURI(reqCtx.GetRequest().URL.Path)
	if key != "id" {
		return ""
	}
	return strings.TrimSpace(value)
}

// providerIDFromPath pulls the `{providerId}` segment out of the
// edit / delete endpoints:
//
//	.../oauth-providers/<providerId>
func providerIDFromPath(path string) string {
	segs := strings.Split(strings.Trim(path, "/"), "/")
	for i := 0; i+1 < len(segs); i++ {
		if segs[i] == "oauth-providers" {
			return strings.TrimSpace(segs[i+1])
		}
	}
	return ""
}

type bagGetter interface {
	Get(key string) (interface{}, bool)
}

// resolveOrgDB resolves the per-org DB for the given application id
// by scanning per-org DBs via OrgDBForApp. Falls back to the manager
// DB when the DI context does not carry a registry (test mode).
func resolveOrgDB(reqCtx request.RequestContext, appID string) (*sql.DB, error) {
	diCtx := reqCtx.GetDI()
	if bag, ok := diCtx.(bagGetter); ok {
		if raw, ok := bag.Get(dbregistry.ContextBagKey); ok {
			if reg, ok := raw.(*dbregistry.OrgUserDBRegistry); ok && reg != nil {
				db, _, err := orgrouter.OrgDBForApp(reg, appID)
				if err != nil {
					return nil, fmt.Errorf("application not found in routing index")
				}
				return db, nil
			}
		}
	}
	// Fallback: use the manager DB directly (test / single-DB mode).
	mgr := diCtx.GetDatabaseManager()
	if mgr == nil || mgr.Connector == nil || mgr.Connector.DB == nil {
		return nil, fmt.Errorf("database manager not available")
	}
	return mgr.Connector.DB, nil
}

// openRepo resolves the per-org DB for the given application and
// wraps it in the providers Repository. Writes an error response
// and returns (nil, false) on failure.
func openRepo(reqCtx request.RequestContext) (*repository.Repository, bool) {
	w := reqCtx.GetWriter()
	appID := appIDFromPath(reqCtx)
	if appID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "application id is required")
		return nil, false
	}
	orgDB, err := resolveOrgDB(reqCtx, appID)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return nil, false
	}
	return repository.New(orgDB), true
}
