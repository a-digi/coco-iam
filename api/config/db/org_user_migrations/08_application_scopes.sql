/***Statement***/
CREATE TABLE IF NOT EXISTS application_scopes (
    id TEXT NOT NULL PRIMARY KEY UNIQUE,
    application_id TEXT NOT NULL,
    scope_id TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    resource_ids TEXT NOT NULL DEFAULT '[]',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    is_active BOOLEAN NOT NULL DEFAULT TRUE
);
/***Statement***/
CREATE UNIQUE INDEX IF NOT EXISTS application_scopes_application_id_scope_id_uindex
    ON application_scopes (application_id, scope_id);
/***Statement***/
CREATE INDEX IF NOT EXISTS application_scopes_application_id_index
    ON application_scopes (application_id);
