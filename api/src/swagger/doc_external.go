// Package swagger provides the embedded swagger spec and documentation stubs
// for external-library handlers that cannot be annotated in their source packages.
package swagger

// The handlers below are implemented by external libraries (coco-queue, coco-observe)
// and wired via shims in config/routes/routes.go. Annotations are placed here as
// documentation stubs so they appear in the generated OpenAPI spec.

// adminQueueStatsDoc documents GET /admin/queue/queues.
//
// @Summary     List queue statistics
// @Description Returns real-time statistics for all registered queues (backlog, workers, throughput).
// @Tags        admin-queue
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} interface{}
// @Failure     401,403,500 {object} map[string]interface{}
// @Router      /admin/queue/queues [get]
func adminQueueStatsDoc() {} //nolint:unused

// adminQueueCreateDoc documents POST /admin/queue/queues.
//
// @Summary     Create a queue
// @Description Creates a new named task queue.
// @Tags        admin-queue
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       body body interface{} true "Queue config"
// @Success     201 {object} interface{}
// @Failure     400,401,403,500 {object} map[string]interface{}
// @Router      /admin/queue/queues [post]
func adminQueueCreateDoc() {} //nolint:unused

// adminQueueTasksListDoc documents GET /admin/queue/queue_tasks.
//
// @Summary     List queue tasks
// @Description Returns a paginated list of tasks across all queues.
// @Tags        admin-queue
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} interface{}
// @Failure     401,403,500 {object} map[string]interface{}
// @Router      /admin/queue/queue_tasks [get]
func adminQueueTasksListDoc() {} //nolint:unused

// adminQueueTaskGetDoc documents GET /admin/queue/queue_tasks/{id}.
//
// @Summary     Get queue task
// @Description Returns details for a single task by ID.
// @Tags        admin-queue
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Task ID"
// @Success     200 {object} interface{}
// @Failure     401,403,404,500 {object} map[string]interface{}
// @Router      /admin/queue/queue_tasks/{id} [get]
func adminQueueTaskGetDoc() {} //nolint:unused

// adminQueueRetryDoc documents POST /admin/queue/retry/{id}.
//
// @Summary     Retry failed task
// @Description Re-queues a dead-lettered or failed task for another execution attempt.
// @Tags        admin-queue
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Task ID"
// @Success     200 {object} interface{}
// @Failure     401,403,404,500 {object} map[string]interface{}
// @Router      /admin/queue/retry/{id} [post]
func adminQueueRetryDoc() {} //nolint:unused

// adminQueueTaskPayloadDoc documents GET /admin/queue/tasks/{id}/payload.
//
// @Summary     Get task payload
// @Description Returns the raw JSON payload of a queued task.
// @Tags        admin-queue
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Task ID"
// @Success     200 {object} interface{}
// @Failure     401,403,404,500 {object} map[string]interface{}
// @Router      /admin/queue/tasks/{id}/payload [get]
func adminQueueTaskPayloadDoc() {} //nolint:unused

// observePushDoc documents POST /admin/observe/push.
//
// @Summary     Push metrics
// @Description Ingests metrics from a coco-observe agent. Public endpoint (HMAC-authenticated at the payload level).
// @Tags        admin-observe
// @Accept      json
// @Produce     json
// @Param       body body interface{} true "Metrics payload"
// @Success     200 {object} interface{}
// @Failure     400,500 {object} map[string]interface{}
// @Router      /admin/observe/push [post]
func observePushDoc() {} //nolint:unused

// observeQueryDoc documents GET /admin/observe/metrics and sub-paths.
//
// @Summary     Query metrics
// @Description Returns aggregated metrics. Variants: /metrics (windowed), /metrics/latest (latest per agent), /metrics/raw (raw events).
// @Tags        admin-observe
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} interface{}
// @Failure     401,403,500 {object} map[string]interface{}
// @Router      /admin/observe/metrics [get]
func observeQueryDoc() {} //nolint:unused

// observeMetricsLatestDoc documents GET /admin/observe/metrics/latest.
//
// @Summary     Query latest metrics
// @Tags        admin-observe
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} interface{}
// @Failure     401,403,500 {object} map[string]interface{}
// @Router      /admin/observe/metrics/latest [get]
func observeMetricsLatestDoc() {} //nolint:unused

// observeMetricsRawDoc documents GET /admin/observe/metrics/raw.
//
// @Summary     Query raw metrics
// @Tags        admin-observe
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} interface{}
// @Failure     401,403,500 {object} map[string]interface{}
// @Router      /admin/observe/metrics/raw [get]
func observeMetricsRawDoc() {} //nolint:unused

// observeAgentsDoc documents GET/POST/PATCH/DELETE /admin/observe/agents.
//
// @Summary     Manage observe agents
// @Description List, create, update, or delete monitoring agents. The GET handler also accepts {id} in the path for single-agent retrieval.
// @Tags        admin-observe
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} interface{}
// @Failure     400,401,403,500 {object} map[string]interface{}
// @Router      /admin/observe/agents [get]
// @Router      /admin/observe/agents [post]
// @Router      /admin/observe/agents/{id} [patch]
// @Router      /admin/observe/agents/{id} [delete]
func observeAgentsDoc() {} //nolint:unused

// observeAgentDownloadDoc documents GET /admin/observe/agents/{id}/download.
//
// @Summary     Download agent binary
// @Description Returns the compiled agent binary for the given agent ID.
// @Tags        admin-observe
// @Produce     application/octet-stream
// @Security    BearerAuth
// @Param       id path string true "Agent ID"
// @Success     200 "Binary file"
// @Failure     401,403,404,500 {object} map[string]interface{}
// @Router      /admin/observe/agents/{id}/download [get]
func observeAgentDownloadDoc() {} //nolint:unused
