// Package admin hosts the admin-session HTTP CRUD for OAuth
// clients registered against a workspace-application. Mounted
// at /api/v1/applications/{id}/oauth-clients in
// route-application-login.yaml.
package admin

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"

	"github.com/a-digi/coco-iam/src/applications/oauthserverwiring"
	"github.com/a-digi/coco-iam/src/oauthserver/sqlstore"
	"github.com/a-digi/coco-iam/src/organizations/users/dbregistry"
	"github.com/a-digi/coco-iam/src/orgrouter"
	"github.com/a-digi/coco-lift/resource/uri"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// appIDFromPath pulls the `{id:<uuid>}` segment out of the URL.
// Mirrors the sibling api-credentials admin handlers.
func appIDFromPath(reqCtx request.RequestContext) string {
	key, value := uri.ExtractKeyAndValueFromURI(reqCtx.GetRequest().URL.Path)
	if key != "id" {
		return ""
	}
	return strings.TrimSpace(value)
}

// clientIDFromPath reads the `{clientRowId}` segment from the
// edit/delete endpoints:
//
//	.../oauth-clients/<rowId>
func clientIDFromPath(path string) string {
	segs := strings.Split(strings.Trim(path, "/"), "/")
	for i := 0; i+1 < len(segs); i++ {
		if segs[i] == "oauth-clients" {
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

// openRepo resolves the per-org DB for the application and wraps
// it in the sqlstore.ClientRepo + the project bcrypt hasher.
// Writes an error response + returns (nil, false) on failure.
func openRepo(reqCtx request.RequestContext) (*sqlstore.ClientRepo, bool) {
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
	return sqlstore.NewClientRepo(orgDB, oauthserverwiring.NewBcryptHasher(0)), true
}
