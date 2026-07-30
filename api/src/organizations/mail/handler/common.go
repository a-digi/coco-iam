// Package handler serves the org-scoped mail settings/accounts/templates
// API under /api/v1/organizations/{id}/mail/*. Mirrors
// api/src/admin/mail's handler shape, scoped to a specific org's own
// users.db instead of the global mail.db. See
// plan/org-app-email-settings/plan.md.
package handler

import (
	"database/sql"
	"net/http"
	"strings"

	"github.com/a-digi/coco-iam/src/organizations/users/dbregistry"
	"github.com/a-digi/coco-iam/src/orgrouter"
	"github.com/a-digi/coco-lift/resource/uri"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

type bagGetter interface {
	Get(key string) (interface{}, bool)
}

// extractAllKeyValues extends uri.ExtractKeyAndValueFromURI (which
// returns only the first {key:value}-wrapped segment) to paths with
// more than one path parameter, e.g.
// /organizations/{id:abc}/mail/accounts/{accountId:xyz} — every route
// in this package needs both the org id and a nested resource id.
func extractAllKeyValues(path string) map[string]string {
	out := map[string]string{}
	for _, seg := range uri.SplitURIPath(path) {
		if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") && !strings.HasPrefix(seg, "{res:") {
			inner := seg[1 : len(seg)-1]
			parts := strings.SplitN(inner, ":", 2)
			if len(parts) == 2 {
				out[parts[0]] = parts[1]
			}
		}
	}
	return out
}

// resolveOrgDB opens the per-org users.db for the org whose id is the
// "id" path parameter — mirrors general/admin.resolveOrgStore, one
// level lower (returns the raw *sql.DB so callers can construct
// whichever mail repo they need).
func resolveOrgDB(reqCtx request.RequestContext) (*sql.DB, bool) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	orgID := strings.TrimSpace(extractAllKeyValues(r.URL.Path)["id"])
	if orgID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "org id missing from path")
		return nil, false
	}

	bag, ok := reqCtx.GetDI().(bagGetter)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "DI context has unexpected type")
		return nil, false
	}
	raw, ok := bag.Get(dbregistry.ContextBagKey)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "org db registry not available")
		return nil, false
	}
	reg, ok := raw.(*dbregistry.OrgUserDBRegistry)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "org db registry has unexpected type")
		return nil, false
	}

	orgDB, err := orgrouter.ForOrg(reg, orgID)
	if err != nil {
		response.ErrorResponse(w, http.StatusNotFound, "organization not found")
		return nil, false
	}
	return orgDB, true
}

// nestedID reads a second path parameter (beyond "id") — e.g.
// "accountId" or "templateId" — writing a 400 response and returning
// ("", false) if it's missing.
func nestedID(reqCtx request.RequestContext, key string) (string, bool) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()
	v := strings.TrimSpace(extractAllKeyValues(r.URL.Path)[key])
	if v == "" {
		response.ErrorResponse(w, http.StatusBadRequest, key+" is required")
		return "", false
	}
	return v, true
}
