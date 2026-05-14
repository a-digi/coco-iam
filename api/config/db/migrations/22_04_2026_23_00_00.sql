/***Statement***/
-- Audit trail for hard-deleted organizations. One row is inserted by
-- the organization-deleted queue consumer for each org the admin
-- removes, capturing a JSON snapshot of the org + its workspaces +
-- applications at the moment of deletion. The DB files themselves
-- (org_<id>.db, org_users_<id>.db) are moved into
-- ./data/db/deleted_databases/ by the same consumer. Admins use this
-- table to answer "what did this org contain" after the fact.
CREATE TABLE IF NOT EXISTS deleted_organizations
(
    organization_id TEXT NOT NULL CONSTRAINT deleted_organizations_pk PRIMARY KEY UNIQUE,
    snapshot_json TEXT NOT NULL,
    deleted_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
/***Statement***/
CREATE INDEX IF NOT EXISTS deleted_organizations_deleted_at_idx
    ON deleted_organizations (deleted_at);
