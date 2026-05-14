/***Statement***/
CREATE TABLE IF NOT EXISTS application_oauth_clients (
    id TEXT NOT NULL CONSTRAINT application_oauth_clients_pk PRIMARY KEY UNIQUE,
    application_id TEXT NOT NULL,
    client_id TEXT NOT NULL,
    client_secret_hash TEXT,
    client_type TEXT NOT NULL DEFAULT 'confidential',
    display_name TEXT NOT NULL DEFAULT '',
    redirect_uris TEXT NOT NULL DEFAULT '[]',
    allowed_scopes TEXT NOT NULL DEFAULT '[]',
    require_consent INTEGER NOT NULL DEFAULT 1,
    access_token_ttl INTEGER NOT NULL DEFAULT 3600,
    refresh_token_ttl INTEGER NOT NULL DEFAULT 1209600,
    is_active INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
/***Statement***/
CREATE UNIQUE INDEX IF NOT EXISTS application_oauth_clients_app_client_id_idx
    ON application_oauth_clients (application_id, client_id);
