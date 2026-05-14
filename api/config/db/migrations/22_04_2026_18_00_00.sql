/***Statement***/
ALTER TABLE workspace ADD COLUMN slug TEXT NOT NULL DEFAULT '';
/***Statement***/
-- Backfill: seed existing rows with an id-derived slug so the
-- upcoming unique index doesn't fire on empty strings sharing an org.
UPDATE workspace SET slug = 'ws-' || substr(id, 1, 8) WHERE slug = '';
/***Statement***/
CREATE UNIQUE INDEX IF NOT EXISTS workspace_org_slug_unique_idx
    ON workspace (organization_id, slug);
/***Statement***/
-- Applications: client_id is no longer globally unique. It is unique
-- only within the owning workspace — the same client_id may coexist
-- in two different workspaces. The old global index is replaced by a
-- composite (workspace_id, client_id) unique index.
DROP INDEX IF EXISTS applications_client_id_uindex;
/***Statement***/
CREATE UNIQUE INDEX IF NOT EXISTS applications_workspace_client_id_unique_idx
    ON applications (workspace_id, client_id);
