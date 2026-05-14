/***Statement***/
CREATE TABLE IF NOT EXISTS application_api_credentials
(
    id              TEXT NOT NULL CONSTRAINT application_api_credentials_pk PRIMARY KEY UNIQUE,
    application_id  TEXT NOT NULL,
    api_id          TEXT NOT NULL,
    secret_hash     TEXT NOT NULL,
    label           TEXT NOT NULL DEFAULT '',
    purposes        TEXT NOT NULL DEFAULT '[]',
    expires_at      DATETIME NOT NULL,
    is_active       BOOLEAN NOT NULL DEFAULT 1,
    last_used_at    DATETIME,
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    revoked_at      DATETIME
);
/***Statement***/
CREATE UNIQUE INDEX IF NOT EXISTS application_api_credentials_api_id_uindex
    ON application_api_credentials (api_id);
/***Statement***/
CREATE INDEX IF NOT EXISTS application_api_credentials_app_idx
    ON application_api_credentials (application_id);
/***Statement***/
CREATE INDEX IF NOT EXISTS application_api_credentials_active_idx
    ON application_api_credentials (is_active);
