// Package admin hosts custom HTTP handlers for the organization_users
// resource — currently the POST create path which needs to trigger the
// activation flow in addition to the generic insert.
package admin

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/a-digi/coco-iam/src/activation"
	"github.com/a-digi/coco-iam/src/organizations/users/dbregistry"
	users_entity "github.com/a-digi/coco-iam/src/organizations/users/entity"
	org_user_query "github.com/a-digi/coco-iam/src/organizations/users/repository/query"
	"github.com/a-digi/coco-iam/src/orgrouter"
	"github.com/a-digi/coco-iam/src/userrules"
	db "github.com/a-digi/coco-orm/orm"

	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

type createPayload struct {
	Username       string `json:"username"`
	Email          string `json:"email"`
	OrganizationID string `json:"organization_id"`
	IsActive       bool   `json:"is_active"`
	// RedirectApplicationID is the UUID of the application the newly-
	// activated user should be sent to. Optional — when empty the
	// activation flow falls back to the admin /login page.
	RedirectApplicationID string `json:"redirect_application_id,omitempty"`
}

type createResponse struct {
	User            *users_entity.User `json:"user"`
	Activation      *activationEcho    `json:"activation,omitempty"`
	ActivationError string             `json:"activation_error,omitempty"`
}

type activationEcho struct {
	ExpiresAt string `json:"expires_at"`
}

// CustomCreateOrganizationUserHandler serves POST /admin/{res:organization_users}.
// Inserts a regular-user row and delegates to the activation service to
// send the invite email. No password is accepted on the wire.
func CustomCreateOrganizationUserHandler(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	ctx := reqCtx.GetDI()

	manager := ctx.GetDatabaseManager()
	if manager == nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "database manager not available")
		return
	}

	var payload createPayload
	if err := reqCtx.BindJSON(&payload); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid json payload: "+err.Error())
		return
	}
	if payload.Username == "" || payload.Email == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "username and email are required")
		return
	}
	if payload.OrganizationID == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "organization_id is required")
		return
	}

	// Apply the org's user-rules to username + email. Violations are
	// user-fixable, so return them verbatim.
	if rsStore := resolveUserRulesStore(ctx); rsStore != nil {
		rs, _ := rsStore.GetForOrg(payload.OrganizationID)
		if v := userrules.Validate(rs, userrules.Input{
			Username: payload.Username,
			Email:    payload.Email,
		}); len(v) > 0 {
			response.ErrorResponse(w, http.StatusBadRequest, strings.Join(v, " "))
			return
		}
	}

	reg := resolveOrgUserRegistry(ctx)
	if reg == nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "org user db registry not available")
		return
	}
	orgDB, err := orgrouter.ForOrg(reg, payload.OrganizationID)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to open org db: "+err.Error())
		return
	}

	qrepo := org_user_query.New(orgDB)
	if exists, err := qrepo.ExistsByUsername(payload.Username); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to check username: "+err.Error())
		return
	} else if exists {
		response.ErrorResponse(w, http.StatusConflict, "username already taken")
		return
	}
	if exists, err := qrepo.ExistsByEmailExcludingID(payload.Email, ""); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to check email: "+err.Error())
		return
	} else if exists {
		response.ErrorResponse(w, http.StatusConflict, "email already taken")
		return
	}

	user := &users_entity.User{
		Username:       payload.Username,
		Email:          payload.Email,
		OrganizationID: payload.OrganizationID,
		IsActive:       payload.IsActive,
	}
	if err := insertUser(manager.Connector.DB, orgDB, user); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to create user: "+err.Error())
		return
	}

	// Provision a default ACL entry so the user can authenticate against
	// the target application. Without this row FindUserForLogin returns
	// ErrNotFound and every login attempt returns 401 regardless of
	// whether the password is correct.
	if payload.RedirectApplicationID != "" {
		if err := insertDefaultAcl(orgDB, payload.RedirectApplicationID, user.ID); err != nil {
			// Non-fatal: admin can grant access from the user edit page.
			_ = err
		}
	}

	out := createResponse{User: user}
	if svc := resolveActivationService(ctx); svc != nil {
		args := activation.StartArgs{
			UserType: activation.UserTypeUser,
			UserID:   user.ID,
			Username: user.Username,
			Email:    user.Email,
			OrgID:    payload.OrganizationID,
		}
		// Resolve the optional redirect target. Failure here is
		// non-fatal — the create still succeeds and the user falls
		// back to the default /login after activation.
		if payload.RedirectApplicationID != "" {
			if target, rerr := resolveRedirectTarget(manager, reg, payload.RedirectApplicationID, payload.OrganizationID); rerr == nil {
				args.Redirect = target
			}
		}
		res, serr := svc.Start(context.Background(), args)
		if serr != nil {
			out.ActivationError = serr.Error()
		} else {
			out.Activation = &activationEcho{ExpiresAt: res.ExpiresAt.Format("2006-01-02T15:04:05Z07:00")}
		}
	} else {
		out.ActivationError = "activation service not available"
	}

	response.SuccessResponse(w, http.StatusCreated, out)
}

// resolveRedirectTarget turns an application UUID into the slug triple
// used by the per-app login URL. Enforces that the app's organization
// chain matches `expectedOrgID` — a mismatched app ID (or one the
// admin doesn't own) is silently rejected with a not-found error, and
// the create path treats it as "no redirect" rather than leaking a
// cross-tenant pointer.
func resolveRedirectTarget(manager *db.DatabaseManager, reg *dbregistry.OrgUserDBRegistry, appID, expectedOrgID string) (*activation.RedirectTarget, error) {
	mainDB := manager.Connector.DB
	if reg == nil {
		return nil, fmt.Errorf("org user db registry not available")
	}
	orgDB, orgUUID, err := orgrouter.OrgDBForApp(reg, appID)
	if err != nil {
		return nil, err
	}
	if expectedOrgID != "" && orgUUID != expectedOrgID {
		return nil, errors.New("application does not belong to organization")
	}
	// Org slug from main DB.
	var orgSlug string
	if err := mainDB.QueryRow(
		`SELECT organization_id FROM organization WHERE id = ? LIMIT 1`,
		orgUUID,
	).Scan(&orgSlug); err != nil {
		return nil, err
	}
	var wsSlug, clientID, wsID string
	if err := orgDB.QueryRow(
		`SELECT workspace_id, client_id FROM applications WHERE id = ? LIMIT 1`,
		appID,
	).Scan(&wsID, &clientID); err != nil {
		return nil, err
	}
	if err := orgDB.QueryRow(
		`SELECT workspace_id FROM workspace WHERE id = ? LIMIT 1`,
		wsID,
	).Scan(&wsSlug); err != nil {
		return nil, err
	}
	if orgSlug == "" || wsSlug == "" || clientID == "" {
		return nil, errors.New("incomplete slug triple")
	}
	return &activation.RedirectTarget{
		OrgSlug:       orgSlug,
		WorkspaceSlug: wsSlug,
		ClientID:      clientID,
	}, nil
}

// insertDefaultAcl provisions an empty ACL entry so the user can
// authenticate against the given application. Without this row
// FindUserForLogin returns ErrNotFound and every login attempt returns
// 401 before the password is ever checked.
func insertDefaultAcl(orgDB *sql.DB, appID, userID string) error {
	id := newUserID()
	_, err := orgDB.Exec(
		`INSERT OR IGNORE INTO application_user_acl (id, application_id, user_id, roles, is_active)
		 VALUES (?, ?, ?, '[]', TRUE)`,
		id, appID, userID,
	)
	if err != nil {
		return fmt.Errorf("insert default acl: %w", err)
	}
	return nil
}

// insertUser writes the user row into the per-org DB.
func insertUser(_ *sql.DB, orgDB *sql.DB, u *users_entity.User) error {
	if u.ID == "" {
		u.ID = newUserID()
	}
	if _, err := orgDB.Exec(
		`INSERT INTO users (id, username, email, is_active) VALUES (?, ?, ?, ?)`,
		u.ID, u.Username, u.Email, u.IsActive,
	); err != nil {
		return fmt.Errorf("insert user into org db: %w", err)
	}
	return nil
}

func newUserID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "00000000-0000-0000-0000-000000000000"
	}
	hx := hex.EncodeToString(buf)
	return fmt.Sprintf("%s-%s-%s-%s-%s", hx[:8], hx[8:12], hx[12:16], hx[16:20], hx[20:32])
}

type bagGetter interface {
	Get(key string) (interface{}, bool)
}

func resolveActivationService(ctx interface{}) *activation.Service {
	bag, ok := ctx.(bagGetter)
	if !ok {
		return nil
	}
	raw, ok := bag.Get(activation.ContextBagKeyService)
	if !ok {
		return nil
	}
	svc, _ := raw.(*activation.Service)
	return svc
}

func resolveOrgUserRegistry(ctx interface{}) *dbregistry.OrgUserDBRegistry {
	bag, ok := ctx.(bagGetter)
	if !ok {
		return nil
	}
	raw, ok := bag.Get(dbregistry.ContextBagKey)
	if !ok {
		return nil
	}
	reg, _ := raw.(*dbregistry.OrgUserDBRegistry)
	return reg
}

func resolveUserRulesStore(ctx interface{}) *userrules.Store {
	bag, ok := ctx.(bagGetter)
	if !ok {
		return nil
	}
	raw, ok := bag.Get(userrules.ContextBagKeyStore)
	if !ok {
		return nil
	}
	store, _ := raw.(*userrules.Store)
	return store
}
