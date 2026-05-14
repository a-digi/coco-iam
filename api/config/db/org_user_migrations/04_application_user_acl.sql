/***Statement***/
-- application_id is a loose reference to the main-DB `applications`
-- table — SQLite FKs don't cross files. Consistency is enforced at
-- the application layer (delete listeners / cascaded app-layer
-- cleanup).
CREATE TABLE IF NOT EXISTS application_user_acl
(
    id TEXT NOT NULL CONSTRAINT application_user_acl_pk PRIMARY KEY UNIQUE,
    application_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    roles JSON NOT NULL,
    grantable_roles TEXT NOT NULL DEFAULT '[]',
    resource_ids TEXT NOT NULL DEFAULT '{}',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    is_active BOOLEAN NOT NULL DEFAULT TRUE
);
/***Statement***/
CREATE UNIQUE INDEX IF NOT EXISTS application_user_acl_application_id_user_id_uindex
    ON application_user_acl (application_id, user_id);
/***Statement***/
CREATE INDEX IF NOT EXISTS application_user_acl_application_id_index
    ON application_user_acl (application_id);
/***Statement***/
CREATE INDEX IF NOT EXISTS application_user_acl_user_id_index
    ON application_user_acl (user_id);
