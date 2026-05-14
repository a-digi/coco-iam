package failedtasks

import (
	"database/sql"
	"net/http"
	"sort"

	"github.com/a-digi/coco-iam/config/di"
	"github.com/a-digi/coco-iam/src/auth/scopecheck"
	"github.com/a-digi/coco-queue"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// RecentTask is a trimmed view of a recently failed queue task.
type RecentTask struct {
	ID        string `json:"id"`
	LastError string `json:"last_error"`
	QueueName string `json:"queue_name"`
	Attempts  int    `json:"attempts"`
	CreatedAt string `json:"created_at"`
}

// AdminDashboardFailedTasksHandler serves GET /api/v1/admin/dashboard/failed-tasks.
type AdminDashboardFailedTasksHandler struct{}

func (h *AdminDashboardFailedTasksHandler) ServeHTTP(reqCtx request.RequestContext) {
	w := reqCtx.GetWriter()
	r := reqCtx.GetRequest()
	ctx := reqCtx.GetDI()

	checker := scopecheck.NewChecker()
	if ok, _ := checker.HasScope(r.Header, "admin:dashboard:read"); !ok {
		response.ErrorResponse(w, http.StatusForbidden, "forbidden")
		return
	}

	bag, ok := ctx.(*di.ContextBag)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "DI context has unexpected type")
		return
	}
	raw, ok := bag.Get(queue.ContextBagKey)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "queue manager not available")
		return
	}
	mgr, ok := raw.(queue.Manager)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "queue manager has unexpected type")
		return
	}

	// Fan-out: each queue has its own DB file, so gather the most
	// recent 5 failed tasks per queue, then globally merge + trim.
	const perQueueLimit = 5
	const globalLimit = 5
	result := []RecentTask{}
	err := mgr.ForEachQueueDB(func(queueName string, db *sql.DB) error {
		rows, err := db.Query(`
			SELECT id, last_error, attempts, created_at
			FROM queue_tasks
			WHERE status = 'failed'
			ORDER BY created_at DESC
			LIMIT ?
		`, perQueueLimit)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var t RecentTask
			if err := rows.Scan(&t.ID, &t.LastError, &t.Attempts, &t.CreatedAt); err != nil {
				return err
			}
			t.QueueName = queueName
			result = append(result, t)
		}
		return rows.Err()
	})
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to load failed tasks: "+err.Error())
		return
	}

	// Most-recent-first merge. CreatedAt is a lexically-comparable
	// timestamp string so direct string sort matches chronological
	// order for both the RFC3339 and "YYYY-MM-DD HH:MM:SS" shapes.
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt > result[j].CreatedAt })
	if len(result) > globalLimit {
		result = result[:globalLimit]
	}

	response.SuccessResponse(w, http.StatusOK, result)
}
