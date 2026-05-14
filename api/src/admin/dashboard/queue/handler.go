package queue

import (
	"database/sql"
	"net/http"
	"sort"

	"github.com/a-digi/coco-iam/config/di"
	iam_queue "github.com/a-digi/coco-queue"
	"github.com/a-digi/coco-iam/src/auth/scopecheck"
	"github.com/a-digi/coco-server/server/request"
	"github.com/a-digi/coco-server/server/response"
)

// QueueStatusCount holds the task count for a single queue status.
type QueueStatusCount struct {
	Status string `json:"status"`
	Count  int    `json:"count"`
}

// QueueBreakdown aggregates task status counts for a single queue.
type QueueBreakdown struct {
	Name    string `json:"name"`
	Success int    `json:"success"`
	Pending int    `json:"pending"`
	Failed  int    `json:"failed"`
	Total   int    `json:"total"`
}

// QueueResponse bundles the global donut breakdown with the top-queues list.
type QueueResponse struct {
	ByStatus  []QueueStatusCount `json:"by_status"`
	TopQueues []QueueBreakdown   `json:"top_queues"`
}

// AdminDashboardQueueHandler serves GET /api/v1/admin/dashboard/queue.
type AdminDashboardQueueHandler struct{}

func (h *AdminDashboardQueueHandler) ServeHTTP(reqCtx request.RequestContext) {
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
	raw, ok := bag.Get(iam_queue.ContextBagKey)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "queue manager not available")
		return
	}
	mgr, ok := raw.(iam_queue.Manager)
	if !ok {
		response.ErrorResponse(w, http.StatusInternalServerError, "queue manager has unexpected type")
		return
	}

	resp := QueueResponse{
		ByStatus:  []QueueStatusCount{},
		TopQueues: []QueueBreakdown{},
	}

	// Fan-out across per-queue DBs. Global donut = sum of per-queue
	// status counts; top-5 = per-queue totals sorted + trimmed.
	totalsByStatus := map[string]int{}
	perQueue := []QueueBreakdown{}

	err := mgr.ForEachQueueDB(func(queueName string, db *sql.DB) error {
		rows, err := db.Query(`SELECT status, COUNT(*) FROM queue_tasks GROUP BY status`)
		if err != nil {
			return err
		}
		breakdown := QueueBreakdown{Name: queueName}
		for rows.Next() {
			var status string
			var n int
			if err := rows.Scan(&status, &n); err != nil {
				rows.Close()
				return err
			}
			totalsByStatus[status] += n
			breakdown.Total += n
			switch status {
			case "completed":
				breakdown.Success += n
			case "pending", "in_progress":
				breakdown.Pending += n
			case "failed", "dead_lettered":
				breakdown.Failed += n
			}
		}
		rows.Close()
		perQueue = append(perQueue, breakdown)
		return nil
	})
	if err != nil {
		response.ErrorResponse(w, http.StatusInternalServerError, "failed to load queue counts: "+err.Error())
		return
	}

	for status, count := range totalsByStatus {
		resp.ByStatus = append(resp.ByStatus, QueueStatusCount{Status: status, Count: count})
	}
	sort.Slice(perQueue, func(i, j int) bool { return perQueue[i].Total > perQueue[j].Total })
	if len(perQueue) > 5 {
		perQueue = perQueue[:5]
	}
	resp.TopQueues = perQueue

	response.SuccessResponse(w, http.StatusOK, resp)
}
