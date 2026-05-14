/***Statement***/
-- Queue tables moved to their own file tree under ./data/db/queue/.
-- main.db no longer owns any queue state. See
-- plan/queue-per-queue-db/plan.md for the full migration.
DROP TABLE IF EXISTS queue_tasks;
/***Statement***/
DROP TABLE IF EXISTS queues;
