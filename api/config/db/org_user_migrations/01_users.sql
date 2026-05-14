/***Statement***/
-- Per-org user table. `organization_id` is implicit from the DB file
-- (org_users_<orgID>.db) so it's not stored here — removing the column
-- eliminates a source of drift.
CREATE TABLE IF NOT EXISTS users
(
    id TEXT NOT NULL CONSTRAINT users_pk PRIMARY KEY UNIQUE,
    username TEXT NOT NULL,
    email TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    must_change_password BOOLEAN NOT NULL DEFAULT FALSE
);
/***Statement***/
CREATE UNIQUE INDEX IF NOT EXISTS users_email_unique_idx ON users (email);
/***Statement***/
CREATE INDEX IF NOT EXISTS users_username_idx ON users (username);
/***Statement***/
CREATE INDEX IF NOT EXISTS users_is_active_idx ON users (is_active);
