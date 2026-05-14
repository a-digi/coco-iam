/***Statement***/
CREATE TABLE IF NOT EXISTS oauth_authorization_codes (
    code                  TEXT NOT NULL CONSTRAINT oauth_authorization_codes_pk PRIMARY KEY,
    client_row_id         TEXT NOT NULL,
    application_id        TEXT NOT NULL,
    user_id               TEXT NOT NULL,
    redirect_uri          TEXT NOT NULL,
    scopes                TEXT NOT NULL DEFAULT '[]',
    code_challenge        TEXT NOT NULL DEFAULT '',
    code_challenge_method TEXT NOT NULL DEFAULT 'S256',
    nonce                 TEXT NOT NULL DEFAULT '',
    created_at            DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
/***Statement***/
CREATE INDEX IF NOT EXISTS oauth_authorization_codes_created_idx ON oauth_authorization_codes (created_at);
