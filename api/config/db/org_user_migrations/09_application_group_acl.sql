/***Statement***/
CREATE TABLE IF NOT EXISTS application_group_acl (
    id TEXT NOT NULL CONSTRAINT application_group_acl_pk PRIMARY KEY UNIQUE,
    application_id TEXT NOT NULL,
    group_id TEXT NOT NULL,
    roles JSON NOT NULL,
    grantable_roles TEXT NOT NULL DEFAULT '[]',
    resource_ids TEXT NOT NULL DEFAULT '{}',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    is_active BOOLEAN NOT NULL DEFAULT TRUE
);
/***Statement***/
CREATE UNIQUE INDEX IF NOT EXISTS application_group_acl_application_id_group_id_uindex
    ON application_group_acl (application_id, group_id);
/***Statement***/
CREATE INDEX IF NOT EXISTS application_group_acl_application_id_idx
    ON application_group_acl (application_id);
/***Statement***/
CREATE INDEX IF NOT EXISTS application_group_acl_group_id_idx
    ON application_group_acl (group_id);
