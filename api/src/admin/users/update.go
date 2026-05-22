package users

import (
	"encoding/json"
	"net/http"

	"github.com/a-digi/coco-lift/resource/uri"
	user_entity "github.com/a-digi/coco-iam/src/admin/users/entity"
	user_query_repository "github.com/a-digi/coco-iam/src/admin/users/repository/query"
	"github.com/a-digi/coco-iam/src/admin/users/validator"
	crypto_bcrypt "github.com/a-digi/coco-iam/src/auth/crypto/bcrypt"
	auth_db "github.com/a-digi/coco-iam/src/auth/database"
	password_entity "github.com/a-digi/coco-iam/src/auth/database/entity"
	auth_query "github.com/a-digi/coco-iam/src/auth/database/repository/query"
	jwt_token "github.com/a-digi/coco-iam/src/auth/security/jwt"
	db "github.com/a-digi/coco-orm/orm"
	"github.com/a-digi/coco-orm/orm/model"
	"github.com/a-digi/coco-orm/orm/orm"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// CustomUpdateUserHandler handles PATCH /admin/{res:users}/{id}.
//
//	@Summary		Update admin user
//	@Description	Partially updates an admin user. Username changes are rejected.
//	@Tags			admin-users
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		string						true	"User ID"
//	@Param			body	body		entity.UpdateUserRequest	true	"Fields to update"
//	@Success		200		{object}	entity.UserSuccess
//	@Failure		400		{object}	response.ErrorBody
//	@Failure		401		{object}	response.ErrorBody
//	@Failure		404		{object}	response.ErrorBody
//	@Failure		409		{object}	response.ErrorBody
//	@Failure		500		{object}	response.ErrorBody
//	@Router			/admin/users/{id} [patch]
func CustomUpdateUserHandler(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	ctx := reqCtx.GetDI()
	r := reqCtx.GetRequest()

	manager := ctx.GetDatabaseManager()
	if manager == nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "database manager not available")
		return
	}

	key, value := uri.ExtractKeyAndValueFromURI(r.URL.Path)
	if key == "" || value == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "path variable is missing")
		return
	}

	var payload map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "invalid json payload: "+err.Error())
		return
	}

	defer r.Body.Close()

	if payload["is_super_admin"] == true {
		if !validator.VerifySuperAdminPrivilege(reqCtx) {
			response.ErrorResponse(w, http.StatusUnauthorized, "unauthorized")
			return
		}
	}

	// Username change is not allowed
	if _, ok := payload["username"]; ok {
		response.ErrorResponse(w, http.StatusBadRequest, "the change of username is not allowed")
		return
	}

	// Email uniqueness check — reject if another admin already owns this email
	if newEmail, ok := payload["email"].(string); ok && newEmail != "" {
		qrepo := user_query_repository.NewAdminUserQueryRepository(manager)
		exists, err := qrepo.ExistsByEmailExcludingID(newEmail, value)
		if err != nil {
			response.ErrorResponse(w, http.StatusInternalServerError, "failed to check email: "+err.Error())
			return
		}
		if exists {
			response.ErrorResponse(w, http.StatusConflict, "email already taken")
			return
		}
	}

	// Hash password if present
	if pwdRaw, ok := payload["password"]; ok {
		pwdStr, isString := pwdRaw.(string)
		if !isString || pwdStr == "" {
			response.ErrorResponse(w, http.StatusBadRequest, "password must be a non-empty string")
			return
		}

		oldPwdRaw, ok := payload["old_password"]
		if !ok {
			response.ErrorResponse(w, http.StatusBadRequest, "old_password is required when changing password")
			return
		}

		oldPwdStr, isString := oldPwdRaw.(string)
		if !isString || oldPwdStr == "" {
			response.ErrorResponse(w, http.StatusBadRequest, "old_password must be a non-empty string")
			return
		}

		pwrepo := auth_query.NewPasswordQueryRepository(manager)
		authenticator := auth_db.NewPasswordAuthenticator(pwrepo)
		validOld, err := authenticator.Verify(value, oldPwdStr)
		if err != nil {
			response.ErrorResponse(w, http.StatusInternalServerError, "error verifying old password: "+err.Error())
			return
		}
		if !validOld {
			response.ErrorResponse(w, http.StatusUnauthorized, "invalid old password")
			return
		}

		hashed, err := crypto_bcrypt.HashPassword(pwdStr, 0)
		if err != nil {
			response.ErrorResponse(w, http.StatusInternalServerError, "failed to hash password: "+err.Error())
			return
		}

		pwBuilder := &orm.UpdateObjectQueryBuilder{}
		pwQuery, pwArgs, err := pwBuilder.BuildFrom(&password_entity.Password{
			Password: hashed,
		}, orm.IdentityBag{
			"user_id": value,
		})
		if err == nil {
			_, err = manager.Connector.DB.Exec(pwQuery, pwArgs...)
			if err != nil {
				response.ErrorResponse(w, http.StatusInternalServerError, "failed to update password: "+err.Error())
				return
			}
		} else {
			response.ErrorResponse(w, http.StatusInternalServerError, "failed to build password query: "+err.Error())
			return
		}

		delete(payload, "password")
		delete(payload, "old_password")
	}

	// Fetch user. Always do this so we can return the complete user entity (and apply patches if needed).
	var users []user_entity.User
	selectQuery := model.SelectQuery{
		Entity: &users,
		Filters: []model.Filter{
			{Column: key, Value: value, Type: model.FilterTypeExactMatch},
		},
		Pagination: model.Pagination{Page: 1, Limit: 1},
	}

	if err := db.FindByQuery(manager, selectQuery); err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to fetch user: "+err.Error())
		return
	}

	if len(users) == 0 {
		response.ErrorResponse(w, http.StatusNotFound, "user not found")
		return
	}

	existingUser := users[0]

	// Guard: prevent self-activation/deactivation
	if _, hasActive := payload["is_active"]; hasActive {
		tokenPayload, jwtErr := jwt_token.CreateJWTTokenFromHeaders(r.Header)
		if jwtErr != nil {
			response.ErrorResponse(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if existingUser.ID == tokenPayload.Sub {
			response.ErrorResponse(w, http.StatusForbidden, "you cannot activate or deactivate yourself")
			return
		}
	}

	// Now update user
	if len(payload) == 0 {
		response.SuccessResponse(w, http.StatusOK, existingUser)
		return
	}

	remainingBytes, _ := json.Marshal(payload)
	if err := json.Unmarshal(remainingBytes, &existingUser); err != nil {
		response.ErrorResponse(w, http.StatusBadRequest, "failed to apply patch: "+err.Error())
		return
	}

	userBuilder := &orm.UpdateObjectQueryBuilder{}
	uQuery, uArgs, uErr := userBuilder.BuildFrom(&existingUser, orm.IdentityBag{
		key: value,
	})
	if uErr != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to build user query: "+uErr.Error())
		return
	}

	_, err := manager.Connector.DB.Exec(uQuery, uArgs...)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to execute user patch: "+err.Error())
		return
	}

	response.SuccessResponse(w, http.StatusOK, existingUser)
}
