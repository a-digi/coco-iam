// Package handler serves the application-scoped mail settings/accounts/
// templates API under /api/v1/applications/{id}/mail/*. Mirrors
// api/src/organizations/mail/handler's shape, one tier deeper: an
// application's mail tables live in its own org's users.db (scoped by
// application_id), so resolving "the DB for this request" means finding
// which org owns the {id} path segment's application, not opening a
// dedicated per-app DB. See plan/org-app-email-settings/plan.md.
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
// /applications/{id:abc}/mail/accounts/{accountId:xyz} — mirrors
// api/src/organizations/mail/handler's own helper of the same name.
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

// resolveAppDB opens the org users.db that owns the application whose
// id is the "id" path parameter, by scanning every known org DB via
// orgrouter.OrgDBForApp — the same lookup already used by
// registrationfields/admin and other per-application admin handlers,
// since applications have no path-visible org id of their own.
func resolveAppDB(reqCtx request.RequestContext) (db *sql.DB, appID string, ok bool) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	appID = strings.TrimSpace(extractAllKeyValues(r.URL.Path)["id"])
	if appID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "application id missing from path")
		return nil, "", false
	}

	bag, ok2 := reqCtx.GetDI().(bagGetter)
	if !ok2 {
		response.ErrorResponse(w, http.StatusInternalServerError, "DI context has unexpected type")
		return nil, "", false
	}
	raw, ok2 := bag.Get(dbregistry.ContextBagKey)
	if !ok2 {
		response.ErrorResponse(w, http.StatusInternalServerError, "org db registry not available")
		return nil, "", false
	}
	reg, ok2 := raw.(*dbregistry.OrgUserDBRegistry)
	if !ok2 {
		response.ErrorResponse(w, http.StatusInternalServerError, "org db registry has unexpected type")
		return nil, "", false
	}

	appDB, _, err := orgrouter.OrgDBForApp(reg, appID)
	if err != nil {
		response.ErrorResponse(w, http.StatusNotFound, "application not found")
		return nil, "", false
	}
	return appDB, appID, true
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
