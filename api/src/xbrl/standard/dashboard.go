package xbrl

import (
	"net/http"
	"strconv"

	"github.com/a-digi/coco-iam/src/xbrl/standard/repository"
	db "github.com/a-digi/coco-orm/orm"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

type DashboardHandler struct{}

func (h *DashboardHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()
	ctx := reqCtx.GetDI()

	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")

	page := db.DefaultPage
	limit := db.DefaultLimit

	if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
		page = p
	}
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
		limit = l
	}

	repo := repository.NewStandardRepository(ctx.GetDatabaseManager())
	results, err := repo.FindByPagination(page, limit)
	if err != nil {
		ctx.GetLogger().Error("DashboardHandler: failed to fetch entries: %v", err)
		response.ErrorResponse(w, http.StatusInternalServerError, "Failed to fetch entries: "+err.Error())

		return
	}

	response.SuccessResponse(w, http.StatusOK, results)
}
