/***Statement***/
-- Registry of ip-attacks.db generations rotated out once the running
-- entry counter crosses the configured threshold. Lives in the main
-- DB deliberately, so it stays queryable regardless of how many times
-- the attacks database itself has been rotated. See
-- plan/ip-attacks-db-archiving/plan.md
CREATE TABLE IF NOT EXISTS ip_attacks_archives
(
    id          TEXT NOT NULL CONSTRAINT ip_attacks_archives_pk PRIMARY KEY,
    file_path   TEXT NOT NULL,
    started_at  DATETIME NOT NULL,
    archived_at DATETIME NOT NULL,
    row_count   INTEGER NOT NULL,
    size_bytes  INTEGER NOT NULL
);
/***Statement***/
CREATE UNIQUE INDEX IF NOT EXISTS ip_attacks_archives_file_path_idx ON ip_attacks_archives (file_path);
/***Statement***/
CREATE INDEX IF NOT EXISTS ip_attacks_archives_archived_at_idx ON ip_attacks_archives (archived_at DESC);
