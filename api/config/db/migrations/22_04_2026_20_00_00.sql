/***Statement***/
ALTER TABLE organization ADD COLUMN organization_id TEXT NOT NULL DEFAULT '';
/***Statement***/
-- Backfill existing rows with id-derived slugs so the upcoming
-- global-unique index doesn't collide on empty strings.
UPDATE organization SET organization_id = 'org-' || substr(id, 1, 8) WHERE organization_id = '';
/***Statement***/
CREATE UNIQUE INDEX IF NOT EXISTS organization_organization_id_unique_idx
    ON organization (organization_id);
