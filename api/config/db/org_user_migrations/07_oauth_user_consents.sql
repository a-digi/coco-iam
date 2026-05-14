/***Statement***/
-- oauth_user_consents — one row per (user, oauth client) pair.
-- Records which scopes the user approved on the consent screen
-- so returning users with unchanged requests skip it. Revoke
-- sets revoked_at, the handler then treats the row as absent
-- and forces the consent screen on the next authorize call.
CREATE TABLE IF NOT EXISTS oauth_user_consents
(
    id              TEXT NOT NULL CONSTRAINT oauth_user_consents_pk PRIMARY KEY UNIQUE,
    user_id         TEXT NOT NULL,
    client_row_id   TEXT NOT NULL,
    granted_scopes  TEXT NOT NULL DEFAULT '[]',
    granted_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    revoked_at      DATETIME
);
/***Statement***/
CREATE UNIQUE INDEX IF NOT EXISTS oauth_user_consents_user_client_idx
    ON oauth_user_consents (user_id, client_row_id);
