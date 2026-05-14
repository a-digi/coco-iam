package users

import (
	"net/http"

	"github.com/a-digi/coco-lift/resource/uri"
	user_entity "github.com/a-digi/coco-iam/src/admin/users/entity"
	"github.com/a-digi/coco-iam/src/admin/users/validator"
	jwt_token "github.com/a-digi/coco-iam/src/auth/security/jwt"
	db "github.com/a-digi/coco-orm/orm"
	"github.com/a-digi/coco-orm/orm/model"
	"github.com/a-digi/coco-orm/orm/orm"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

func CustomDeleteUserHandler(reqCtx request.RequestContext) {
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

	// Fetch existing user
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

	if existingUser.IsSuperAdmin {
		if !validator.VerifySuperAdminPrivilege(reqCtx) {
			response.ErrorResponse(w, http.StatusForbidden, "unauthorized to delete super admins")
			return
		}

		var superAdmins []user_entity.User
		selectSuperAdminsQuery := model.SelectQuery{
			Entity: &superAdmins,
			Filters: []model.Filter{
				{Column: "is_super_admin", Value: true, Type: model.FilterTypeExactMatch},
			},
		}

		if err := db.FindByQuery(manager, selectSuperAdminsQuery); err != nil {
			response.ErrorResponse(w, http.StatusInternalServerError, "failed to fetch super admins: "+err.Error())
			return
		}

		if len(superAdmins) <= 1 {
			response.ErrorResponse(w, http.StatusForbidden, "cannot delete the last super admin")
			return
		}
	}

	tokenPayload, err := jwt_token.CreateJWTTokenFromHeaders(r.Header)
	if err != nil {
		response.ErrorResponse(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if existingUser.ID == tokenPayload.Sub {
		response.ErrorResponse(w, http.StatusForbidden, "you cannot delete yourself")
		return
	}

	builder := &orm.DeleteObjectQueryBuilder{}
	identity := orm.IdentityBag{
		key: value,
	}

	query, args, err := builder.BuildFrom(&existingUser, identity)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to build delete query: "+err.Error())
		return
	}

	if manager.Connector == nil || manager.Connector.DB == nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "database connection is nil")
		return
	}

	_, err = manager.Connector.DB.Exec(query, args...)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to execute delete: "+err.Error())
		return
	}

	response.SuccessResponse(w, http.StatusOK, existingUser)
}
