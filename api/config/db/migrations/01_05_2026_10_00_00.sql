/***Statement***/
CREATE TABLE IF NOT EXISTS oauth_token_org_index (
    token  TEXT NOT NULL CONSTRAINT oauth_token_org_index_pk PRIMARY KEY UNIQUE,
    org_id TEXT NOT NULL
);
/***Statement***/
CREATE INDEX IF NOT EXISTS oauth_token_org_index_org_idx ON oauth_token_org_index (org_id);
/***Statement***/
CREATE TABLE IF NOT EXISTS application_org_index (
    application_id TEXT NOT NULL CONSTRAINT application_org_index_pk PRIMARY KEY UNIQUE,
    org_id TEXT NOT NULL
);
/***Statement***/
INSERT OR IGNORE INTO oauth_token_org_index (token, org_id)
SELECT r.state, i.org_id FROM oauth_auth_requests r
JOIN application_org_index i ON i.application_id = r.application_id;
/***Statement***/
INSERT OR IGNORE INTO oauth_token_org_index (token, org_id)
SELECT c.code, i.org_id FROM oauth_authorization_codes c
JOIN application_org_index i ON i.application_id = c.application_id;
/***Statement***/
INSERT OR IGNORE INTO oauth_token_org_index (token, org_id)
SELECT r.token_hash, i.org_id FROM oauth_refresh_tokens r
JOIN application_org_index i ON i.application_id = r.application_id;
