/***Statement***/
-- One row per admin-console login attempt (success or failure),
-- against the admin oauth/authenticate and oauth/verify-mfa
-- endpoints. admin_user_id is NULL when the typed username never
-- resolved to a real admin account. success is 1/0. failure_reason is
-- one of invalid_credentials, inactive_user, no_scopes, mfa_required,
-- mfa_failed, or NULL on success. Passwords are never stored here. See
-- plan/login-audit-log/plan.md Step 2.
CREATE TABLE IF NOT EXISTS admin_login_attempts
(
    id             TEXT NOT NULL CONSTRAINT admin_login_attempts_pk PRIMARY KEY,
    admin_user_id  TEXT,
    username       TEXT NOT NULL,
    success        INTEGER NOT NULL,
    failure_reason TEXT,
    ip             TEXT NOT NULL,
    user_agent     TEXT,
    created_at     DATETIME NOT NULL
);
/***Statement***/
CREATE INDEX IF NOT EXISTS admin_login_attempts_admin_user_id_idx ON admin_login_attempts (admin_user_id);
/***Statement***/
CREATE INDEX IF NOT EXISTS admin_login_attempts_created_at_idx ON admin_login_attempts (created_at DESC);
/***Statement***/
-- Running total of rows inserted across this generation's tables,
-- checked against the archive threshold without a full COUNT scan.
-- Reset to 0 whenever the archiver creates a fresh generation. See
-- plan/ip-attacks-db-archiving/plan.md (the same mechanism, reused —
-- see plan/login-audit-log/plan.md Step 1).
CREATE TABLE IF NOT EXISTS db_meta
(
    key   TEXT NOT NULL CONSTRAINT db_meta_pk PRIMARY KEY,
    value TEXT NOT NULL
);
/***Statement***/
INSERT INTO db_meta (key, value)
VALUES ('entry_count', '0');
