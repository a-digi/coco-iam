/***Statement***/
-- Registry of admin_login.db generations rotated out once the running
-- entry counter crosses the configured threshold. Lives in the main
-- DB deliberately, so it stays queryable regardless of how many times
-- admin_login.db itself has been rotated. Same mechanism as
-- ip_attacks_archives — see plan/ip-attacks-db-archiving/plan.md and
-- plan/login-audit-log/plan.md Step 2.
CREATE TABLE IF NOT EXISTS admin_login_archives
(
    id          TEXT NOT NULL CONSTRAINT admin_login_archives_pk PRIMARY KEY,
    file_path   TEXT NOT NULL,
    started_at  DATETIME NOT NULL,
    archived_at DATETIME NOT NULL,
    row_count   INTEGER NOT NULL,
    size_bytes  INTEGER NOT NULL
);
/***Statement***/
CREATE UNIQUE INDEX IF NOT EXISTS admin_login_archives_file_path_idx ON admin_login_archives (file_path);
/***Statement***/
CREATE INDEX IF NOT EXISTS admin_login_archives_archived_at_idx ON admin_login_archives (archived_at DESC);
