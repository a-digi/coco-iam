/***Statement***/
CREATE TABLE IF NOT EXISTS oauth_refresh_tokens (
    id             TEXT NOT NULL CONSTRAINT oauth_refresh_tokens_pk PRIMARY KEY UNIQUE,
    token_hash     TEXT NOT NULL UNIQUE,
    client_row_id  TEXT NOT NULL,
    application_id TEXT NOT NULL,
    user_id        TEXT NOT NULL,
    scopes         TEXT NOT NULL DEFAULT '[]',
    issued_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at     DATETIME NOT NULL,
    revoked_at     DATETIME,
    replaced_by_id TEXT
);
/***Statement***/
CREATE INDEX IF NOT EXISTS oauth_refresh_tokens_user_client_idx ON oauth_refresh_tokens (user_id, client_row_id);
/***Statement***/
CREATE INDEX IF NOT EXISTS oauth_refresh_tokens_token_hash_idx ON oauth_refresh_tokens (token_hash);
