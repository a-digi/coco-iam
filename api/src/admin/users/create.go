package users

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"

	"github.com/a-digi/coco-iam/src/activation"
	user_entity "github.com/a-digi/coco-iam/src/admin/users/entity"
	user_persistent_repository "github.com/a-digi/coco-iam/src/admin/users/repository/persistent"
	user_query_repository "github.com/a-digi/coco-iam/src/admin/users/repository/query"
	"github.com/a-digi/coco-iam/src/admin/users/validator"
	"github.com/a-digi/coco-iam/src/userrules"
	crypto_bcrypt "github.com/a-digi/coco-iam/src/auth/crypto/bcrypt"
	password_entity "github.com/a-digi/coco-iam/src/auth/database/entity"
	password_persistent_repository "github.com/a-digi/coco-iam/src/auth/database/repository/persistent"
	db "github.com/a-digi/coco-orm/orm"
)

type AdminUserCreator struct {
	AdminUserRepository *user_persistent_repository.AdminUserPersistentRepository
	PasswordRepository  *password_persistent_repository.PasswordPersistentRepository
	QueryRepository     *user_query_repository.AdminUserQueryRepository
}

func NewAdminUserCreator(manager *db.DatabaseManager) *AdminUserCreator {
	return &AdminUserCreator{
		AdminUserRepository: user_persistent_repository.NewAdminUserPersistentRepository(manager),
		PasswordRepository:  password_persistent_repository.NewPasswordPersistentRepository(manager),
		QueryRepository:     user_query_repository.NewAdminUserQueryRepository(manager),
	}
}

// UserPayload is what the admin create endpoint now accepts. Password is
// intentionally absent: every new admin goes through the activation flow
// and receives a temporary password via email. CLI bootstrap paths
// continue to use the in-package `Create(...)` helper which still accepts
// a password.
type UserPayload struct {
	Username     string `json:"username"`
	Email        string `json:"email"`
	IsActive     bool   `json:"is_active"`
	IsSuperAdmin bool   `json:"is_super_admin"`
}

type createResponse struct {
	User            *user_entity.User `json:"user"`
	Activation      *activationEcho   `json:"activation,omitempty"`
	ActivationError string            `json:"activation_error,omitempty"`
}

type activationEcho struct {
	ExpiresAt string `json:"expires_at"`
}

// CustomCreateUserHandler handles POST /admin/{res:users}. Inserts the
// admin user row (without a password), then delegates to the activation
// service which generates the token + temp password and enqueues the
// invite email.
func CustomCreateUserHandler(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	ctx := reqCtx.GetDI()

	manager := ctx.GetDatabaseManager()
	if manager == nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "database manager not available")
		return
	}

	var payload UserPayload
	if err := reqCtx.BindJSON(&payload); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid json payload: "+err.Error())
		return
	}
	if payload.IsSuperAdmin {
		if !validator.VerifySuperAdminPrivilege(reqCtx) {
			response.ErrorResponse(w, http.StatusUnauthorized, "unauthorized")
			return
		}
	}

	// Apply admin user-rules to username + email before creating. Rule
	// violations are user-fixable, so surface them plainly.
	if rsStore := resolveUserRulesStore(ctx); rsStore != nil {
		rs, _ := rsStore.GetAdmin()
		if v := userrules.Validate(rs, userrules.Input{
			Username: payload.Username,
			Email:    payload.Email,
		}); len(v) > 0 {
			response.ErrorResponse(w, http.StatusBadRequest, strings.Join(v, " "))
			return
		}
	}

	creator := NewAdminUserCreator(manager)
	user, err := creator.CreatePending(payload.Username, payload.Email, payload.IsActive, payload.IsSuperAdmin)
	if err != nil {
		switch err.Error() {
		case "username already taken", "email already taken":
			response.ErrorResponse(w, http.StatusConflict, err.Error())
		default:
			response.ErrorResponse(w, http.StatusInternalServerError, "failed to create user: "+err.Error())
		}
		return
	}

	out := createResponse{User: user}
	if svc := resolveActivationService(ctx); svc != nil {
		res, serr := svc.Start(context.Background(), activation.StartArgs{
			UserType: activation.UserTypeAdmin,
			UserID:   user.ID,
			Username: user.Username,
			Email:    user.Email,
		})
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

// Create keeps the original password-taking signature so CLI bootstrap
// and tests that rely on it continue to work. For HTTP-driven creation
// use CreatePending + the activation flow instead.
func (c *AdminUserCreator) Create(username string, email string, password string, isActive bool, isSuperAdmin bool) (*user_entity.User, error) {
	if c == nil || c.AdminUserRepository == nil || c.PasswordRepository == nil {
		return nil, errors.New("repositories not initialized")
	}
	if username == "" {
		return nil, errors.New("username is required")
	}
	if email == "" {
		return nil, errors.New("email is required")
	}
	if password == "" {
		return nil, errors.New("password is required")
	}

	if c.QueryRepository != nil {
		if exists, err := c.QueryRepository.ExistsByUsername(username); err != nil {
			return nil, err
		} else if exists {
			return nil, errors.New("username already taken")
		}
		if exists, err := c.QueryRepository.ExistsByEmailExcludingID(email, ""); err != nil {
			return nil, err
		} else if exists {
			return nil, errors.New("email already taken")
		}
	}

	hashed, err := crypto_bcrypt.HashPassword(password, 0)
	if err != nil {
		return nil, err
	}

	user := &user_entity.User{
		Username:     username,
		Email:        email,
		IsActive:     isActive,
		IsSuperAdmin: isSuperAdmin,
	}

	if err := c.AdminUserRepository.Insert(user); err != nil {
		return nil, err
	}

	pw := &password_entity.AdminPassword{
		UserId:   user.ID,
		Password: hashed,
	}

	if err := c.PasswordRepository.InsertAdmin(pw); err != nil {
		return nil, err
	}

	return user, nil
}

// CreatePending inserts the admin user without a password. The activation
// service is expected to follow up with a temp password + email.
func (c *AdminUserCreator) CreatePending(username, email string, isActive, isSuperAdmin bool) (*user_entity.User, error) {
	if c == nil || c.AdminUserRepository == nil {
		return nil, errors.New("repositories not initialized")
	}
	if username == "" {
		return nil, errors.New("username is required")
	}
	if email == "" {
		return nil, errors.New("email is required")
	}

	if c.QueryRepository != nil {
		if exists, err := c.QueryRepository.ExistsByUsername(username); err != nil {
			return nil, err
		} else if exists {
			return nil, errors.New("username already taken")
		}
		if exists, err := c.QueryRepository.ExistsByEmailExcludingID(email, ""); err != nil {
			return nil, err
		} else if exists {
			return nil, errors.New("email already taken")
		}
	}

	user := &user_entity.User{
		Username:     username,
		Email:        email,
		IsActive:     isActive,
		IsSuperAdmin: isSuperAdmin,
	}
	if err := c.AdminUserRepository.Insert(user); err != nil {
		return nil, err
	}
	return user, nil
}

// bagGetter is a narrow local interface that ContextBag satisfies —
// keeps this file out of the config/di import path so handlers can be
// referenced from the resource registry without an import cycle.
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
