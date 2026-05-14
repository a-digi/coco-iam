package userprofile

import (
	"crypto/rsa"
	"net/http"
	"time"

	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// GetMeHandler serves
//
//	GET /a/{orgSlug}/{wsSlug}/{appSlug}/profile/me
//
// Thin orchestrator: parse URL → resolve slugs → authenticate →
// load profile → respond. All decision logic lives in
// `authenticateUser` (pure) and `BuildResponse` (pure). The four
// collaborators are interface-typed (ports.go) so tests substitute
// fakes without constructing the real DI bag.
//
// Instantiated by routes.go with production adapters (see
// adapters.go). A nil Now field falls back to time.Now so callers
// don't have to set it unless they want deterministic behaviour.
type GetMeHandler struct {
	Slugs    SlugResolver
	Keys     KeyLoader
	Users    UserOrgReader
	Profiles ProfileReader
	Now      func() time.Time
}

const genericUnauthorized = "unauthorized"

type meResponse struct {
	Fields []FieldWithValue `json:"fields"`
}

func (h *GetMeHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	orgSlug, wsSlug, appSlug, ok := parseSlugSegments(r.URL.Path)
	if !ok {
		response.ErrorResponse(w, http.StatusUnauthorized, genericUnauthorized)
		return
	}

	appID, orgID, err := h.Slugs.ResolveSlugs(orgSlug, wsSlug, appSlug)
	if err != nil {
		// Unknown slug triple — obfuscated as generic 401 so the
		// endpoint can't be used as a slug-enumeration oracle.
		response.ErrorResponse(w, http.StatusUnauthorized, genericUnauthorized)
		return
	}

	// Close over the resolved appID so authenticateUser's signature
	// stays free of it. KeyLoader accepts (appID, kid); we supply
	// appID once here.
	loadKey := LoadPublicKeyFunc(func(kid string) (*rsa.PublicKey, error) {
		return h.Keys.LoadPublicKey(appID, kid)
	})
	userOrg := UserOrgLookupFunc(h.Users.UserOrg)

	nowFn := time.Now
	if h.Now != nil {
		nowFn = h.Now
	}

	userID, authErr := authenticateUser(
		r.Header.Get("Authorization"),
		orgID,
		loadKey,
		userOrg,
		nowFn(),
	)
	if authErr != nil {
		if authErr.Status == http.StatusInternalServerError {
			response.ErrorResponse(w, http.StatusInternalServerError, genericUnauthorized)
			return
		}
		response.ErrorResponse(w, http.StatusUnauthorized, genericUnauthorized)
		return
	}

	fields, data, err := h.Profiles.LoadProfile(orgID, userID)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to load profile")
		return
	}

	response.SuccessResponse(w, http.StatusOK, meResponse{
		Fields: BuildResponse(fields, data),
	})
}

// -- slug parsing (duplicated from sibling /a/... packages so this
// package's import surface stays narrow) ----------------------------

func parseSlugSegments(path string) (org, ws, app string, ok bool) {
	parts := splitSegments(path)
	if len(parts) < 5 {
		return "", "", "", false
	}
	if parts[0] != "a" {
		return "", "", "", false
	}
	return parts[1], parts[2], parts[3], true
}

func splitSegments(path string) []string {
	out := make([]string, 0, 8)
	start := 0
	for i := 0; i <= len(path); i++ {
		if i == len(path) || path[i] == '/' {
			if i > start {
				out = append(out, path[start:i])
			}
			start = i + 1
		}
	}
	return out
}
