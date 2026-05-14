/***Statement***/
-- application_oauth_clients — one row per (application, registered
-- third-party client). Each row lets the client identified by
-- client_id authenticate users of the referenced application via
-- the OIDC authorization-code flow. The client_secret is hashed
-- with bcrypt. client_type=public stores NULL for confidential
-- clients that use PKCE exclusively.
CREATE TABLE IF NOT EXISTS application_oauth_clients
(
    id                   TEXT NOT NULL CONSTRAINT application_oauth_clients_pk PRIMARY KEY UNIQUE,
    application_id       TEXT NOT NULL,
    client_id            TEXT NOT NULL,
    client_secret_hash   TEXT,
    client_type          TEXT NOT NULL DEFAULT 'confidential',
    display_name         TEXT NOT NULL DEFAULT '',
    redirect_uris        TEXT NOT NULL DEFAULT '[]',
    allowed_scopes       TEXT NOT NULL DEFAULT '[]',
    require_consent      INTEGER NOT NULL DEFAULT 1,
    access_token_ttl     INTEGER NOT NULL DEFAULT 3600,
    refresh_token_ttl    INTEGER NOT NULL DEFAULT 1209600,
    is_active            INTEGER NOT NULL DEFAULT 1,
    created_at           DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at           DATETIME DEFAULT CURRENT_TIMESTAMP
);
/***Statement***/
CREATE UNIQUE INDEX IF NOT EXISTS application_oauth_clients_app_client_id_idx
    ON application_oauth_clients (application_id, client_id);
/***Statement***/
-- oauth_authorization_codes — short-lived in-flight codes. TTL
-- is 5 minutes, enforced by the handler. A periodic sweeper
-- removes expired rows. Codes are single-use and deleted on
-- exchange.
CREATE TABLE IF NOT EXISTS oauth_authorization_codes
(
    code                   TEXT NOT NULL CONSTRAINT oauth_authorization_codes_pk PRIMARY KEY UNIQUE,
    client_row_id          TEXT NOT NULL,
    application_id         TEXT NOT NULL,
    user_id                TEXT NOT NULL,
    redirect_uri           TEXT NOT NULL,
    scopes                 TEXT NOT NULL DEFAULT '[]',
    code_challenge         TEXT NOT NULL DEFAULT '',
    code_challenge_method  TEXT NOT NULL DEFAULT 'S256',
    nonce                  TEXT NOT NULL DEFAULT '',
    created_at             DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
/***Statement***/
CREATE INDEX IF NOT EXISTS oauth_authorization_codes_created_idx
    ON oauth_authorization_codes (created_at);
/***Statement***/
-- oauth_refresh_tokens — opaque refresh tokens. token_hash is
-- SHA-256 of the raw token value so a DB snapshot cannot leak
-- usable credentials. replaced_by_id tracks rotation chains so
-- replay of a consumed token reveals whole-family compromise.
CREATE TABLE IF NOT EXISTS oauth_refresh_tokens
(
    id              TEXT NOT NULL CONSTRAINT oauth_refresh_tokens_pk PRIMARY KEY UNIQUE,
    token_hash      TEXT NOT NULL UNIQUE,
    client_row_id   TEXT NOT NULL,
    application_id  TEXT NOT NULL,
    user_id         TEXT NOT NULL,
    scopes          TEXT NOT NULL DEFAULT '[]',
    issued_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at      DATETIME NOT NULL,
    revoked_at      DATETIME,
    replaced_by_id  TEXT
);
/***Statement***/
CREATE INDEX IF NOT EXISTS oauth_refresh_tokens_user_client_idx
    ON oauth_refresh_tokens (user_id, client_row_id);
