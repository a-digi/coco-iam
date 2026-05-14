/***Statement***/
CREATE TABLE IF NOT EXISTS oauth_auth_requests (
    state          TEXT NOT NULL CONSTRAINT oauth_auth_requests_pk PRIMARY KEY,
    application_id TEXT NOT NULL,
    provider       TEXT NOT NULL,
    code_verifier  TEXT NOT NULL,
    return_url     TEXT NOT NULL DEFAULT '',
    created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
/***Statement***/
CREATE INDEX IF NOT EXISTS oauth_auth_requests_created_idx ON oauth_auth_requests (created_at);
