/***Statement***/
-- Registry of rotated-out <slug>_login.db generations, shared across
-- every application in this org (filtered by application_id) rather
-- than one table per application. Lives in the org's own users.db,
-- not the application's login-log database itself, so it stays
-- queryable regardless of how many times that database has been
-- rotated. See plan/login-audit-log/plan.md Step 6.
CREATE TABLE IF NOT EXISTS application_login_archives
(
    id             TEXT NOT NULL CONSTRAINT application_login_archives_pk PRIMARY KEY,
    application_id TEXT NOT NULL,
    file_path      TEXT NOT NULL,
    started_at     DATETIME NOT NULL,
    archived_at    DATETIME NOT NULL,
    row_count      INTEGER NOT NULL,
    size_bytes     INTEGER NOT NULL
);
/***Statement***/
CREATE INDEX IF NOT EXISTS application_login_archives_application_id_idx ON application_login_archives (application_id);
/***Statement***/
CREATE INDEX IF NOT EXISTS application_login_archives_archived_at_idx ON application_login_archives (archived_at DESC);
