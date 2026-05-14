/***Statement***/
-- Per-queue task table. Each queue has its own SQLite file at
-- ./data/db/queue/<id>_<name>.db, so the `queue_name` column that
-- lived on the shared main-DB `queue_tasks` is redundant here — the
-- queue is implicit from the file.
CREATE TABLE IF NOT EXISTS queue_tasks
(
    id TEXT PRIMARY KEY,
    status TEXT NOT NULL DEFAULT 'pending',
    attempts INTEGER NOT NULL DEFAULT 0,
    max_attempts INTEGER NOT NULL DEFAULT 3,
    last_error TEXT NOT NULL DEFAULT '',
    next_attempt_at TEXT NOT NULL DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    completed_at TEXT NOT NULL DEFAULT ''
);
/***Statement***/
CREATE INDEX IF NOT EXISTS queue_tasks_status_idx ON queue_tasks (status);
/***Statement***/
CREATE INDEX IF NOT EXISTS queue_tasks_next_attempt_at_idx ON queue_tasks (next_attempt_at);
/***Statement***/
CREATE INDEX IF NOT EXISTS queue_tasks_created_at_idx ON queue_tasks (created_at);
