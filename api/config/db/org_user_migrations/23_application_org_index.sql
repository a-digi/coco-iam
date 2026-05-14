/***Statement***/
CREATE TABLE IF NOT EXISTS application_org_index (
    application_id TEXT NOT NULL CONSTRAINT application_org_index_pk PRIMARY KEY UNIQUE,
    org_id         TEXT NOT NULL
);
/***Statement***/
CREATE INDEX IF NOT EXISTS application_org_index_org_idx ON application_org_index (org_id);
