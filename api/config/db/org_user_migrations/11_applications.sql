/***Statement***/
CREATE TABLE IF NOT EXISTS applications (
    id TEXT NOT NULL CONSTRAINT applications_pk PRIMARY KEY UNIQUE,
    workspace_id TEXT NOT NULL,
    client_id TEXT NOT NULL,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    allow_recovery BOOLEAN NOT NULL DEFAULT 1,
    allow_registration BOOLEAN NOT NULL DEFAULT 0,
    registration_type TEXT NOT NULL DEFAULT 'legacy',
    allow_password_login BOOLEAN NOT NULL DEFAULT 1
);
/***Statement***/
CREATE UNIQUE INDEX IF NOT EXISTS applications_client_id_uindex ON applications (client_id);
/***Statement***/
CREATE INDEX IF NOT EXISTS applications_workspace_id_index ON applications (workspace_id);
/***Statement***/
CREATE INDEX IF NOT EXISTS applications_id_index ON applications (id);
/***Statement***/
CREATE UNIQUE INDEX IF NOT EXISTS applications_workspace_client_id_uindex
    ON applications (workspace_id, client_id);
