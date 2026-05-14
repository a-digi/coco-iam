/***Statement***/
CREATE TABLE IF NOT EXISTS workspace_org_index (
    workspace_id TEXT NOT NULL CONSTRAINT workspace_org_index_pk PRIMARY KEY UNIQUE,
    org_id TEXT NOT NULL
);
/***Statement***/
CREATE INDEX IF NOT EXISTS workspace_org_index_org_idx ON workspace_org_index (org_id);
/***Statement***/
CREATE TABLE IF NOT EXISTS application_org_index (
    application_id TEXT NOT NULL CONSTRAINT application_org_index_pk PRIMARY KEY UNIQUE,
    org_id TEXT NOT NULL
);
/***Statement***/
CREATE INDEX IF NOT EXISTS application_org_index_org_idx ON application_org_index (org_id);
/***Statement***/
INSERT OR IGNORE INTO workspace_org_index (workspace_id, org_id)
SELECT id, organization_id FROM workspace
WHERE organization_id IS NOT NULL AND organization_id != '';
/***Statement***/
INSERT OR IGNORE INTO application_org_index (application_id, org_id)
SELECT a.id, w.organization_id FROM applications a
JOIN workspace w ON w.id = a.workspace_id
WHERE w.organization_id IS NOT NULL AND w.organization_id != '';
