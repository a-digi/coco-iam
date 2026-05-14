package users

import (
	"context"
	"net/http"

	"github.com/a-digi/coco-lift/resource/uri"
	"github.com/a-digi/coco-iam/src/activation"
	user_entity "github.com/a-digi/coco-iam/src/admin/users/entity"
	jwt_token "github.com/a-digi/coco-iam/src/auth/security/jwt"
	db "github.com/a-digi/coco-orm/orm"
	"github.com/a-digi/coco-orm/orm/model"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

type AdminUserSendActivationHandler struct{}

func (h *AdminUserSendActivationHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	ctx := reqCtx.GetDI()
	r := reqCtx.GetRequest()

	manager := ctx.GetDatabaseManager()
	if manager == nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "database manager not available")
		return
	}

	_, id := uri.ExtractKeyAndValueFromURI(r.URL.Path)
	if id == "" {
		response.ErrorResponse(w, http.StatusBadRequest, "user id missing from path")
		return
	}

	var users []user_entity.User
	selectQuery := model.SelectQuery{
		Entity: &users,
		Filters: []model.Filter{
			{Column: "id", Value: id, Type: model.FilterTypeExactMatch},
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
	user := users[0]

	if user.IsActive {
		response.ErrorResponse(w, http.StatusBadRequest, "user account is already active")
		return
	}

	tokenPayload, err := jwt_token.CreateJWTTokenFromHeaders(r.Header)
	if err != nil {
		response.ErrorResponse(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if user.ID == tokenPayload.Sub {
		response.ErrorResponse(w, http.StatusForbidden, "you cannot send an activation email to yourself")
		return
	}

	svc := resolveActivationService(ctx)
	if svc == nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "activation service not available")
		return
	}

	res, serr := svc.Start(context.Background(), activation.StartArgs{
		UserType: activation.UserTypeAdmin,
		UserID:   user.ID,
		Username: user.Username,
		Email:    user.Email,
	})
	if serr != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to send activation email: "+serr.Error())
		return
	}

	response.SuccessResponse(w, http.StatusAccepted, map[string]string{
		"status":     "sent",
		"expires_at": res.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}
