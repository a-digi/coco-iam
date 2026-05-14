// Package admin hosts the admin-session HTTP handlers for
// application API credentials. Mounted under /api/v1/applications/{id}/api-credentials
// in route-application-login.yaml. Contrast with the sibling `public`
// package, which hosts the slug-routed machine-auth endpoints under
// /a/{orgSlug}/{wsSlug}/{appSlug}/... .
package admin

import (
	"errors"
	"net/http"
	"strings"

	"github.com/a-digi/coco-lift/resource/uri"

	"github.com/a-digi/coco-iam/src/applications/apicredentials/dbregistry"
	"github.com/a-digi/coco-iam/src/applications/apicredentials/repository"
	users_dbregistry "github.com/a-digi/coco-iam/src/organizations/users/dbregistry"
	"github.com/a-digi/coco-iam/src/orgrouter"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// appIDFromPath pulls the `{id:<uuid>}` segment out of the URL.
// Matches the pattern used by the sibling /keys handlers.
func appIDFromPath(reqCtx request.RequestContext) string {
	key, value := uri.ExtractKeyAndValueFromURI(reqCtx.GetRequest().URL.Path)
	if key != "id" {
		return ""
	}
	return strings.TrimSpace(value)
}

// credIDFromPath pulls the `{credId}` segment out of the revoke
// endpoint's URL — `.../api-credentials/<credId>/revoke`.
func credIDFromPath(path string) string {
	// Expected shape: [... "api-credentials", "<credId>", "revoke"]
	segs := strings.Split(strings.Trim(path, "/"), "/")
	for i := 0; i+2 < len(segs); i++ {
		if segs[i] == "api-credentials" && segs[i+2] == "revoke" {
			return strings.TrimSpace(segs[i+1])
		}
	}
	return ""
}

// bagGetter is the minimal slice of di.ContextBag the resolvers need.
type bagGetter interface {
	Get(key string) (interface{}, bool)
}

// resolveCredRegistry fetches the per-org credentials DB registry
// from the DI bag. On failure writes an error response + returns nil.
func resolveCredRegistry(reqCtx request.RequestContext) *dbregistry.OrgApiCredentialsDBRegistry {
	w := reqCtx.GetWriter()
	ctx := reqCtx.GetDI()
	bag, ok := ctx.(bagGetter)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "DI context has unexpected type")
		return nil
	}
	raw, ok := bag.Get(dbregistry.ContextBagKey)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "api-credentials registry not available")
		return nil
	}
	reg, ok := raw.(*dbregistry.OrgApiCredentialsDBRegistry)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "api-credentials registry has unexpected type")
		return nil
	}
	return reg
}

// ErrAppNotFound signals the URL's application id doesn't map to any org.
var ErrAppNotFound = errors.New("api-credentials admin: application not found")

// resolveOrgIDForApp scans per-org DBs to find the one that owns appID.
func resolveOrgIDForApp(reqCtx request.RequestContext, appID string) (string, error) {
	bag, ok := reqCtx.GetDI().(bagGetter)
	if !ok {
		return "", errors.New("api-credentials admin: DI context not keyed")
	}
	raw, ok := bag.Get(users_dbregistry.ContextBagKey)
	if !ok {
		return "", errors.New("api-credentials admin: users registry not in DI")
	}
	reg, ok := raw.(*users_dbregistry.OrgUserDBRegistry)
	if !ok {
		return "", errors.New("api-credentials admin: users registry type mismatch")
	}
	_, orgID, err := orgrouter.OrgDBForApp(reg, appID)
	if err != nil {
		return "", ErrAppNotFound
	}
	return orgID, nil
}

// openRepoForApp is the common "resolve the org → open the per-org DB
// → wrap in a Repository" flow used by all three admin handlers.
// Returns (repo, orgID, true) on success, or writes an error response
// and returns ok=false on failure.
func openRepoForApp(reqCtx request.RequestContext, appID string) (repo *repository.Repository, orgID string, ok bool) {
	w := reqCtx.GetWriter()
	orgID, err := resolveOrgIDForApp(reqCtx, appID)
	if err != nil {
		if errors.Is(err, ErrAppNotFound) {
			response.ErrorResponse(w, http.StatusNotFound, "application not found")
			return nil, "", false
		}
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return nil, "", false
	}
	registry := resolveCredRegistry(reqCtx)
	if registry == nil {
		return nil, "", false
	}
	credDB, err := registry.For(orgID)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return nil, "", false
	}
	return repository.New(credDB.Connector.DB), orgID, true
}
