/***Statement***/
-- Queue registry. One row per logical queue, holding the admin-chosen
-- name + the UUID used to derive the per-queue DB filename
-- (./data/db/queue/<id>_<name>.db). Small, global, rarely written.
CREATE TABLE IF NOT EXISTS queues
(
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
/***Statement***/
CREATE INDEX IF NOT EXISTS queues_name_idx ON queues (name);
/***Statement***/
-- Lightweight task-to-queue lookup. Admin endpoints that accept a
-- bare task id (retry, payload download) use this to discover which
-- per-queue DB file to open. One row per task, deleted by the queue
-- manager when the task row is pruned from its per-queue file. A
-- dangling row is not catastrophic — it surfaces as a 404 on lookup,
-- which is the same response a genuinely-missing task would return.
CREATE TABLE IF NOT EXISTS tasks_index
(
    task_id TEXT PRIMARY KEY,
    queue_name TEXT NOT NULL,
    queue_id TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
/***Statement***/
CREATE INDEX IF NOT EXISTS tasks_index_queue_name_idx ON tasks_index (queue_name);
