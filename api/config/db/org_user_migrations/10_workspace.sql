/***Statement***/
CREATE TABLE IF NOT EXISTS workspace (
    id TEXT NOT NULL CONSTRAINT workspace_pk PRIMARY KEY,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    organization_id TEXT,
    workspace_id TEXT NOT NULL DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    is_active BOOLEAN NOT NULL DEFAULT TRUE
);
/***Statement***/
CREATE INDEX IF NOT EXISTS workspace_id_index ON workspace (id);
/***Statement***/
CREATE INDEX IF NOT EXISTS workspace_organization_id_idx ON workspace (organization_id);
/***Statement***/
CREATE UNIQUE INDEX IF NOT EXISTS workspace_org_workspace_id_unique_idx
    ON workspace (organization_id, workspace_id);
