package authentication

import (
	"encoding/json"
	"net/http"

	"github.com/a-digi/coco-iam/config"
	user_acl_repo "github.com/a-digi/coco-iam/src/admin/acl/repository"
	passwordexpiry "github.com/a-digi/coco-iam/src/admin/users/passwordexpiry"
	user_repository "github.com/a-digi/coco-iam/src/admin/users/repository/query"
	auth_db "github.com/a-digi/coco-iam/src/auth/database"
	auth_entity "github.com/a-digi/coco-iam/src/auth/database/entity"
	auth_query "github.com/a-digi/coco-iam/src/auth/database/repository/query"
	"github.com/a-digi/coco-iam/src/auth/oauth"
	oauth_lib "github.com/a-digi/coco-oauth/oauth"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// DatabaseAuthenticationLogin is the admin-only login endpoint. It
// authoritatively queries admin_users and never `users`. A non-admin
// identity that happens to share a username/email with some admin
// can never authenticate here — the lookup is by (username, role),
// not by credentials guessed against a union of tables. Organisation
// users authenticate through /api/v1/organizations/slug/{slug}/authenticate
// instead. Keep this invariant: do not extend this handler to fall
// back to `users` under any condition.
type DatabaseAuthenticationLogin struct{}

func (h *DatabaseAuthenticationLogin) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()
	ctx := reqCtx.GetDI()

	var creds auth_entity.LoginCredentials
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(&creds); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if creds.Username == "" || creds.Password == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "username and password are required")
		return
	}

	manager := ctx.GetDatabaseManager()
	if manager == nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "database manager not available")
		return
	}

	urepo := user_repository.NewAdminUserQueryRepository(manager)
	user, found, err := urepo.FindByUsername(creds.Username)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "user lookup failed")
		return
	}
	if !found || user == nil || !user.IsActive {
		response.ErrorResponse(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	pwrepo := auth_query.NewAdminPasswordQueryRepository(manager)
	authenticator := auth_db.NewPasswordAuthenticator(pwrepo)
	ok2, err := authenticator.Verify(user.ID, creds.Password)

	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "authentication error")
		return
	}

	if !ok2 {
		response.ErrorResponse(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	cfgBytes, err := config.ReadConfigFile("config.json")

	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "auth config error")
		return
	}

	cfg, err := oauth_lib.LoadAuthConfigFromBytes(cfgBytes)

	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "auth config error")
		return
	}

	// Users with a pending password change get a JWT whose only scope is
	// `system:pwd_reset_required`. Every protected route requires a
	// normal scope, so the user is automatically 403'd everywhere
	// except the change-password endpoint.
	var mustChange bool
	if err := manager.Connector.DB.QueryRow(
		`SELECT must_change_password FROM admin_users WHERE id = ?`, user.ID,
	).Scan(&mustChange); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to read account state")
		return
	}

	if !mustChange {
		if checker := resolveAdminExpiryChecker(ctx); checker != nil {
			if expired, err := checker.IsExpired(user.ID); err == nil && expired {
				mustChange = true
			}
		}
	}

	var scopes []string
	if mustChange {
		scopes = []string{"system:pwd_reset_required"}
	} else {
		aclRepo := user_acl_repo.NewUserAclRepository(manager)
		scopes, err = aclRepo.FindUserScopes(user.ID)
		if err != nil {
			response.ErrorResponse(w, http.StatusInternalServerError, "failed to get user scopes")
			return
		}
		if len(scopes) == 0 {
			response.ErrorResponse(w, http.StatusForbidden, "No access rights have been assigned to your account. Please contact your administrator.")
			return
		}
	}

	tokenResp, err := oauth.IssueToken(cfg, user.ID, scopes)

	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "token signing failed")
		return
	}

	response.SuccessResponse(w, http.StatusOK, tokenResp)
}

func resolveAdminExpiryChecker(ctx interface{}) *passwordexpiry.Checker {
	bag, ok := ctx.(interface{ Get(string) (interface{}, bool) })
	if !ok {
		return nil
	}
	raw, ok := bag.Get("passwordexpiry.AdminChecker")
	if !ok {
		return nil
	}
	c, _ := raw.(*passwordexpiry.Checker)
	return c
}
