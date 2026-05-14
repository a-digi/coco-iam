package me

import (
	"net/http"

	user_query "github.com/a-digi/coco-iam/src/admin/users/repository/query"
	jwt_token "github.com/a-digi/coco-iam/src/auth/security/jwt"
	"github.com/a-digi/coco-oauth/oauth"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

type MeAclHandler struct{}

func (h *MeAclHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()
	ctx := reqCtx.GetDI()

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		response.ErrorResponse(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	token, err := oauth.ExtractBearer(authHeader)
	if err != nil {
		response.ErrorResponse(w, http.StatusUnauthorized, "invalid token format")
		return
	}

	userId, err := jwt_token.ParseJWTSubject(token)
	if err != nil || userId == "" {
		response.ErrorResponse(w, http.StatusUnauthorized, "failed to parse user from token")
		return
	}

	manager := ctx.GetDatabaseManager()
	if manager == nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "database manager not available")
		return
	}

	repo := user_query.NewAdminUserQueryRepository(manager)
	data, err := repo.GetMeAcl(userId)
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to fetch user acl")
		return
	}

	response.SuccessResponse(w, http.StatusOK, data)
}
