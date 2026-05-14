/***Statement***/
-- application_oauth_providers — one row per (application, external
-- IdP). Drives the "Continue with Google / GitHub / Microsoft"
-- buttons on the workspace-application login page and the
-- authorize / callback handshake on the backend.
--
-- client_secret_enc stores AES-256-GCM ciphertext of the
-- third-party provider client secret, encrypted with the master
-- key from env var COCO_IAM_MASTER_KEY. The secret never leaves
-- the backend in plaintext after the admin POSTs it.
--
-- allow_login / allow_registration split so admins can offer
-- returning-user login via a provider without letting it create
-- new accounts.
CREATE TABLE IF NOT EXISTS application_oauth_providers
(
    id                  TEXT NOT NULL CONSTRAINT application_oauth_providers_pk PRIMARY KEY UNIQUE,
    application_id      TEXT NOT NULL,
    provider            TEXT NOT NULL,
    client_id           TEXT NOT NULL,
    client_secret_enc   TEXT NOT NULL,
    discovery_url       TEXT NOT NULL DEFAULT '',
    authorize_url       TEXT NOT NULL DEFAULT '',
    token_url           TEXT NOT NULL DEFAULT '',
    userinfo_url        TEXT NOT NULL DEFAULT '',
    scopes              TEXT NOT NULL DEFAULT '',
    allow_login         INTEGER NOT NULL DEFAULT 1,
    allow_registration  INTEGER NOT NULL DEFAULT 0,
    is_active           INTEGER NOT NULL DEFAULT 1,
    created_at          DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at          DATETIME DEFAULT CURRENT_TIMESTAMP
);
/***Statement***/
CREATE UNIQUE INDEX IF NOT EXISTS application_oauth_providers_app_provider_idx
    ON application_oauth_providers (application_id, provider);
/***Statement***/
-- oauth_auth_requests — short-lived in-flight authorize handshake
-- state. Holds the random state parameter, PKCE verifier and the
-- requested return_url. Rows are deleted on callback use and
-- TTL-swept after 10 minutes by the handler.
CREATE TABLE IF NOT EXISTS oauth_auth_requests
(
    state            TEXT NOT NULL CONSTRAINT oauth_auth_requests_pk PRIMARY KEY UNIQUE,
    application_id   TEXT NOT NULL,
    provider         TEXT NOT NULL,
    code_verifier    TEXT NOT NULL,
    return_url       TEXT NOT NULL,
    created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
/***Statement***/
CREATE INDEX IF NOT EXISTS oauth_auth_requests_created_idx
    ON oauth_auth_requests (created_at);
