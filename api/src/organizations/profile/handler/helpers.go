// Package handler houses the HTTP handlers for the organization profile
// subsystem (both field administration and per-user value CRUD).
package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/a-digi/coco-iam/src/organizations/profile"
	"github.com/a-digi/coco-iam/src/organizations/profile/dbregistry"
	"github.com/a-digi/coco-iam/src/auth/security/jwt"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// extractToken scans the path for a `{key:value}` segment and returns `value`.
// The coco-lift URL convention sends entity ids wrapped this way, e.g.
//
//	/organizations/{res:organizations}/{id:abc-123}/profile-fields/{fieldId:xyz}
//
// extractToken(path, "id") returns "abc-123".
// extractToken(path, "fieldId") returns "xyz".
// Returns "" when the key is not present.
func extractToken(path, key string) string {
	segments := strings.Split(strings.Trim(path, "/"), "/")
	prefix := "{" + key + ":"
	for _, seg := range segments {
		if strings.HasPrefix(seg, prefix) && strings.HasSuffix(seg, "}") {
			return seg[len(prefix) : len(seg)-1]
		}
	}
	return ""
}

// extractOrgID pulls the organization id out of an organizations-scoped URL.
// Expects the standard `{id:<orgId>}` token.
func extractOrgID(path string) string {
	return extractToken(path, "id")
}

type bagGetter interface {
	Get(key string) (interface{}, bool)
}

// repositoryFromRequest opens the per-org DB and returns a profile.Repository.
// Returns the org id (for convenience) along with any error.
func repositoryFromRequest(reqCtx request.RequestContext) (string, *profile.Repository, error) {
	r := reqCtx.GetRequest()
	orgID := extractOrgID(r.URL.Path)
	if orgID == "" {
		return "", nil, errBadRequest("organization id is required")
	}
	bag, ok := reqCtx.GetDI().(bagGetter)
	if !ok {
		return orgID, nil, errInternal("di context does not expose Get")
	}
	regAny, ok := bag.Get(dbregistry.ContextBagKey)
	if !ok {
		return orgID, nil, errInternal("org db registry not registered")
	}
	registry, ok := regAny.(*dbregistry.OrgDBRegistry)
	if !ok {
		return orgID, nil, errInternal("org db registry has unexpected type")
	}
	mgr, err := registry.For(orgID)
	if err != nil {
		return orgID, nil, errInternal("failed to open org db: " + err.Error())
	}
	return orgID, profile.NewRepository(mgr), nil
}

// callerID returns the authenticated user id from the JWT. Empty when missing.
func callerID(reqCtx request.RequestContext) string {
	payload, err := jwt.CreateJWTTokenFromHeaders(reqCtx.GetRequest().Header)
	if err != nil {
		return ""
	}
	return payload.Sub
}

// decodeJSONBody reads the request body into the passed pointer.
func decodeJSONBody(reqCtx request.RequestContext, out interface{}) error {
	body, err := io.ReadAll(reqCtx.GetRequest().Body)
	if err != nil {
		return errBadRequest("could not read body: " + err.Error())
	}
	if len(body) == 0 {
		return errBadRequest("empty body")
	}
	if err := json.Unmarshal(body, out); err != nil {
		return errBadRequest("invalid json: " + err.Error())
	}
	return nil
}

type httpErr struct {
	code int
	msg  string
}

func (e *httpErr) Error() string { return e.msg }

func errBadRequest(msg string) error { return &httpErr{http.StatusBadRequest, msg} }
func errInternal(msg string) error   { return &httpErr{http.StatusInternalServerError, msg} }

// writeErr sends the HTTP error mapped from the given error. Falls back to 500.
func writeErr(w http.ResponseWriter, err error) {
	if e, ok := err.(*httpErr); ok {
		response.ErrorResponse(w, e.code, e.msg)
		return
	}
	response.ErrorResponse(w, http.StatusInternalServerError, err.Error())
}
