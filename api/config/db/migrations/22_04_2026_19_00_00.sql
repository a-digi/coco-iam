/***Statement***/
-- Rename workspace.slug → workspace.workspace_id for naming parity
-- with applications.client_id. Both tables now expose a human-
-- readable, admin-chosen identifier in a predictably-named column.
ALTER TABLE workspace RENAME COLUMN slug TO workspace_id;
/***Statement***/
DROP INDEX IF EXISTS workspace_org_slug_unique_idx;
/***Statement***/
CREATE UNIQUE INDEX IF NOT EXISTS workspace_org_workspace_id_unique_idx
    ON workspace (organization_id, workspace_id);
