package profilefields

import (
	"crypto/rsa"
	"net/http"
	"time"

	userprofile "github.com/a-digi/coco-iam/src/applications/userprofile"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

const genericUnauthorized = "unauthorized"

// ProfileFieldSchema is the wire shape for one profile field definition.
// AcceptMime and MaxBytes are intentionally excluded — those are
// upload-implementation details, not form-rendering hints.
type ProfileFieldSchema struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Label       string   `json:"label"`
	Description string   `json:"description,omitempty"`
	DataType    string   `json:"data_type"`
	IsRequired  bool     `json:"is_required"`
	MinValue    *int     `json:"min_value,omitempty"`
	MaxValue    *int     `json:"max_value,omitempty"`
	Options     []string `json:"options,omitempty"`
	Regex       string   `json:"regex,omitempty"`
	OrderIndex  int      `json:"order_index"`
}

// GetProfileFieldsResponse is the envelope returned by GET /profile/fields.
type GetProfileFieldsResponse struct {
	Fields []ProfileFieldSchema `json:"fields"`
}

// GetProfileFieldsHandler serves
//
//	GET /a/{orgSlug}/{wsSlug}/{appSlug}/profile/fields
//
// Returns the org's active profile field definitions. The RS256 Bearer token
// is validated (same as /profile/me) to confirm the caller is a legitimate
// user of the app; no user-specific data is returned.
//
// A nil Now field falls back to time.Now.
type GetProfileFieldsHandler struct {
	Slugs  SlugResolver
	Keys   KeyLoader
	Users  UserOrgReader
	Fields FieldSchemaReader
	Now    func() time.Time
}

// @Summary     Get organization profile field schema
// @Description Returns active profile field definitions for the organization.
//
//	Requires a valid RS256 Bearer token (same token used by /profile/me).
//	Expired, invalid, or missing tokens receive 401.
//
// @Tags     app-profile-me
// @Produce  json
// @Param    orgSlug  path  string  true  "Organization slug"
// @Param    wsSlug   path  string  true  "Workspace slug"
// @Param    appSlug  path  string  true  "Application slug"
// @Success  200  {object}  GetProfileFieldsResponse
// @Failure  401  {object}  response.ErrorBody
// @Failure  500  {object}  response.ErrorBody
// @Router   /a/{orgSlug}/{wsSlug}/{appSlug}/profile/fields [get]
func (h *GetProfileFieldsHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	orgSlug, wsSlug, appSlug, ok := parseSlugSegments(r.URL.Path)
	if !ok {
		response.ErrorResponse(w, http.StatusUnauthorized, genericUnauthorized)
		return
	}

	appID, orgID, err := h.Slugs.ResolveSlugs(orgSlug, wsSlug, appSlug)
	if err != nil {
		response.ErrorResponse(w, http.StatusUnauthorized, genericUnauthorized)
		return
	}

	loadKey := userprofile.LoadPublicKeyFunc(func(kid string) (*rsa.PublicKey, error) {
		return h.Keys.LoadPublicKey(appID, kid)
	})
	userOrg := userprofile.UserOrgLookupFunc(h.Users.UserOrg)

	nowFn := time.Now
	if h.Now != nil {
		nowFn = h.Now
	}

	_, authErr := userprofile.AuthenticateUser(
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

	fields, err := h.Fields.ActiveFields(orgID)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to load profile fields")
		return
	}

	response.SuccessResponse(w, http.StatusOK, GetProfileFieldsResponse{Fields: fields})
}

// -- slug parsing (mirrors userprofile package; kept local to avoid
// expanding this package's import surface) ----------------------------

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
