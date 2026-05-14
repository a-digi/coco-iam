package authentication

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/a-digi/coco-iam/src/applications/loginpage"
	oauth_entity "github.com/a-digi/coco-iam/src/applications/oauthproviders/entity"
	"github.com/a-digi/coco-iam/src/applications/oauthproviders/repository"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// AppLoginMethodsHandler serves POST
// /api/v1/public/applications/auth-methods?org=<org>&ws=<ws>&app=<app>.
//
// The two-step login UI calls this after the visitor submits their
// identifier. The response lists which authentication methods are
// available for the next step ("password" and any configured
// external OAuth providers).
//
// Invariant: the response is intentionally identical regardless of
// whether the identifier matches a known user. Users are never
// enumerable through this surface. A future SSO routing change that
// narrows the methods based on identifier must preserve this
// contract (e.g. only narrow when the organization has SSO enforced
// for all of its users, so every identifier gets the same reply).
type AppLoginMethodsHandler struct{}

type authMethodsBody struct {
	Identifier string `json:"identifier"`
}

// OAuthProviderListing is a per-provider row in the auth-methods
// response. Contains everything the frontend needs to render
// "Continue with X" buttons without hardcoding provider metadata.
type OAuthProviderListing struct {
	Provider     string `json:"provider"`
	DisplayName  string `json:"display_name"`
	AuthorizeURL string `json:"authorize_url"`
}

type authMethodsResponse struct {
	// Methods lists the auth factors available to this caller at
	// the next step, in the order the FE should present them.
	// "password" is always present; "oauth" is included when the
	// application has at least one active OAuth provider.
	Methods []string `json:"methods"`
	// OAuthProviders lists the configured external IdPs. Each
	// entry carries its own authorize URL so the frontend can
	// link straight to the handshake without hardcoding routes.
	OAuthProviders []OAuthProviderListing `json:"oauth_providers,omitempty"`
}

func (h *AppLoginMethodsHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()

	orgSlug := strings.TrimSpace(r.URL.Query().Get("org"))
	wsSlug := strings.TrimSpace(r.URL.Query().Get("ws"))
	clientID := strings.TrimSpace(r.URL.Query().Get("app"))
	if orgSlug == "" || wsSlug == "" || clientID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "missing org, ws, or app query parameter")
		return
	}

	// Body is parsed so a client sending malformed JSON gets a 400,
	// but the identifier value is intentionally unused today — see
	// the package-level invariant above.
	var body authMethodsBody
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	defer r.Body.Close()
	body.Identifier = strings.TrimSpace(body.Identifier)
	if body.Identifier == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "identifier is required")
		return
	}

	loginSvc := resolveLoginPageService(reqCtx)
	providers := listActiveOAuthProviders(reqCtx, orgSlug, wsSlug, clientID, loginSvc)

	methods := []string{}
	// Resolve the app id once so we can check the password-login
	// flag. Failures to resolve silently keep password enabled —
	// the password flow itself enforces the flag, so the worst
	// case of a miss here is the SPA showing the password field
	// and the server rejecting the credentials with "invalid
	// credentials".
	var appID string
	var appOrgID string
	if loginSvc != nil {
		if info, err := loginSvc.Store.FindBySlugs(orgSlug, wsSlug, clientID); err == nil {
			appID = info.ID
			appOrgID = info.OrganizationID
		}
	}
	// Applications now live in the per-org DB. Resolve the org DB and
	// pass it to passwordLoginAllowed. Fall through to password=allowed
	// on any resolution failure so a registry miss doesn't lock users out.
	var orgDB *sql.DB
	if appID != "" && appOrgID != "" {
		if reg := resolveOrgUserRegistry(reqCtx); reg != nil {
			if odb, err := resolveOrgDB(reg, appOrgID); err == nil {
				orgDB = odb
			}
		}
	}
	if appID == "" || passwordLoginAllowed(orgDB, appID) {
		methods = append(methods, "password")
	}
	if len(providers) > 0 {
		methods = append(methods, "oauth")
	}
	response.SuccessResponse(w, http.StatusOK, authMethodsResponse{
		Methods:        methods,
		OAuthProviders: providers,
	})
}

// listActiveOAuthProviders resolves the (org, ws, app) triple via
// loginpage.Service and returns the enabled external IdPs for the
// corresponding application. Failures silently return an empty
// list — the feature is additive; a broken provider list must
// never break password login.
func listActiveOAuthProviders(reqCtx request.RequestContext, orgSlug, wsSlug, appSlug string, loginSvc *loginpage.Service) []OAuthProviderListing {
	if loginSvc == nil {
		return nil
	}
	info, err := loginSvc.Store.FindBySlugs(orgSlug, wsSlug, appSlug)
	if err != nil {
		return nil
	}
	// application_oauth_providers now lives in the per-org DB. Resolve
	// the org DB for this application using the OrgUserDBRegistry.
	var orgDB *sql.DB
	if info.OrganizationID != "" {
		if reg := resolveOrgUserRegistry(reqCtx); reg != nil {
			if odb, oerr := resolveOrgDB(reg, info.OrganizationID); oerr == nil {
				orgDB = odb
			}
		}
	}
	if orgDB == nil {
		return nil
	}
	repo := repository.New(orgDB)
	rows, err := repo.ListForApp(info.ID)
	if err != nil {
		return nil
	}
	out := make([]OAuthProviderListing, 0, len(rows))
	for _, row := range rows {
		if !row.IsActive || !row.AllowLogin {
			continue
		}
		out = append(out, OAuthProviderListing{
			Provider:    string(row.Provider),
			DisplayName: displayNameFor(row.Provider),
			AuthorizeURL: buildAuthorizeURL(orgSlug, wsSlug, appSlug, string(row.Provider)),
		})
	}
	return out
}

func displayNameFor(p oauth_entity.Provider) string {
	switch p {
	case oauth_entity.ProviderGoogle:
		return "Google"
	case oauth_entity.ProviderGitHub:
		return "GitHub"
	case oauth_entity.ProviderMicrosoft:
		return "Microsoft"
	}
	return string(p)
}

// buildAuthorizeURL returns the relative URL the frontend should
// navigate to. Relative path keeps it compatible with any base
// URL the SPA is served from; the backend's route pattern is
// fixed.
func buildAuthorizeURL(orgSlug, wsSlug, appSlug, provider string) string {
	return "/a/" + orgSlug + "/" + wsSlug + "/" + appSlug + "/auth/oauth/" + provider + "/authorize"
}

