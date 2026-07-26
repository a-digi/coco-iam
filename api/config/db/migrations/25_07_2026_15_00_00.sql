/***Statement***/
CREATE TABLE IF NOT EXISTS admin_user_mfa
(
    admin_user_id   TEXT NOT NULL CONSTRAINT admin_user_mfa_pk PRIMARY KEY,
    secret_enc      TEXT NOT NULL,
    is_enabled      BOOLEAN NOT NULL DEFAULT FALSE,
    enrolled_at     DATETIME,
    confirmed_at    DATETIME,
    failed_attempts INTEGER NOT NULL DEFAULT 0,
    locked_until    DATETIME,
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);
/***Statement***/
CREATE TABLE IF NOT EXISTS admin_user_mfa_recovery_codes
(
    id            TEXT NOT NULL CONSTRAINT admin_user_mfa_recovery_codes_pk PRIMARY KEY,
    admin_user_id TEXT NOT NULL,
    code_hash     TEXT NOT NULL,
    used_at       DATETIME,
    created_at    DATETIME DEFAULT CURRENT_TIMESTAMP
);
/***Statement***/
CREATE INDEX IF NOT EXISTS admin_user_mfa_recovery_codes_admin_user_id_idx
    ON admin_user_mfa_recovery_codes (admin_user_id);
