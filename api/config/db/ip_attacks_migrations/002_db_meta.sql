/***Statement***/
-- Running total of rows inserted across this generation's tables,
-- checked against the archive threshold without a full COUNT scan.
-- Reset to 0 whenever the archiver creates a fresh generation. See
-- plan/ip-attacks-db-archiving/plan.md
CREATE TABLE IF NOT EXISTS db_meta
(
    key   TEXT NOT NULL CONSTRAINT db_meta_pk PRIMARY KEY,
    value TEXT NOT NULL
);
/***Statement***/
INSERT INTO db_meta (key, value)
VALUES ('entry_count', '0');
