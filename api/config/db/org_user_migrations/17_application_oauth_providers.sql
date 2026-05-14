/***Statement***/
CREATE TABLE IF NOT EXISTS application_oauth_providers (
    id TEXT NOT NULL CONSTRAINT application_oauth_providers_pk PRIMARY KEY UNIQUE,
    application_id TEXT NOT NULL,
    provider TEXT NOT NULL,
    client_id TEXT NOT NULL,
    client_secret_enc TEXT NOT NULL,
    discovery_url TEXT NOT NULL DEFAULT '',
    authorize_url TEXT NOT NULL DEFAULT '',
    token_url TEXT NOT NULL DEFAULT '',
    userinfo_url TEXT NOT NULL DEFAULT '',
    scopes TEXT NOT NULL DEFAULT '',
    allow_login INTEGER NOT NULL DEFAULT 1,
    allow_registration INTEGER NOT NULL DEFAULT 0,
    is_active INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
/***Statement***/
CREATE UNIQUE INDEX IF NOT EXISTS application_oauth_providers_app_provider_idx
    ON application_oauth_providers (application_id, provider);
