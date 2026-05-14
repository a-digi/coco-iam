// Package admin hosts the admin-session HTTP handlers for the
// application registration schema (steps + fields). Mounted under
// /api/v1/applications/{id}/registration-fields in
// route-application-login.yaml.
package admin

import (
	"errors"
	"net/http"
	"strings"

	"github.com/a-digi/coco-lift/resource/uri"

	"github.com/a-digi/coco-iam/src/applications/registrationfields/repository"
	profile_dbregistry "github.com/a-digi/coco-iam/src/organizations/profile/dbregistry"
	users_dbregistry "github.com/a-digi/coco-iam/src/organizations/users/dbregistry"
	"github.com/a-digi/coco-iam/src/orgrouter"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// ErrAppNotFound signals the URL's application id doesn't map to
// any row in the applications table.
var ErrAppNotFound = errors.New("registrationfields admin: application not found")

// appIDFromPath pulls the `{id:<uuid>}` segment from the URL.
func appIDFromPath(reqCtx request.RequestContext) string {
	key, value := uri.ExtractKeyAndValueFromURI(reqCtx.GetRequest().URL.Path)
	if key != "id" {
		return ""
	}
	return strings.TrimSpace(value)
}

type bagGetter interface {
	Get(key string) (interface{}, bool)
}

// resolveOrgIDForApp scans the per-org DBs to find the one that owns appID
// via its application_org_index table.
func resolveOrgIDForApp(reqCtx request.RequestContext, appID string) (string, error) {
	bag, ok := reqCtx.GetDI().(bagGetter)
	if !ok {
		return "", errors.New("registrationfields admin: DI context not keyed")
	}
	raw, ok := bag.Get(users_dbregistry.ContextBagKey)
	if !ok {
		return "", errors.New("registrationfields admin: users registry not in DI")
	}
	reg, ok := raw.(*users_dbregistry.OrgUserDBRegistry)
	if !ok {
		return "", errors.New("registrationfields admin: users registry type mismatch")
	}
	_, orgID, err := orgrouter.OrgDBForApp(reg, appID)
	if err != nil {
		return "", ErrAppNotFound
	}
	return orgID, nil
}

// resolveProfileRegistry fetches the per-org profile registry from
// the DI bag. profiles.db holds both profile_fields and the two
// new registration tables — one registry covers all three.
func resolveProfileRegistry(reqCtx request.RequestContext) *profile_dbregistry.OrgDBRegistry {
	bag, ok := reqCtx.GetDI().(bagGetter)
	if !ok {
		return nil
	}
	raw, ok := bag.Get(profile_dbregistry.ContextBagKey)
	if !ok {
		return nil
	}
	reg, _ := raw.(*profile_dbregistry.OrgDBRegistry)
	return reg
}

// openRepo resolves orgID → opens the org's profiles.db → wraps
// in a Repository. On any failure writes an error response and
// returns ok=false so the caller bails.
func openRepo(reqCtx request.RequestContext, appID string) (*repository.Repository, string, bool) {
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
	reg := resolveProfileRegistry(reqCtx)
	if reg == nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "profile registry not available")
		return nil, "", false
	}
	db, err := reg.For(orgID)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return nil, "", false
	}
	return repository.New(db.Connector.DB), orgID, true
}
