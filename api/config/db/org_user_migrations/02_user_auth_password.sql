/***Statement***/
CREATE TABLE IF NOT EXISTS user_auth_password
(
    user_id TEXT NOT NULL CONSTRAINT user_auth_password_pk PRIMARY KEY UNIQUE,
    password TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    is_active BOOLEAN NOT NULL DEFAULT TRUE
);
/***Statement***/
CREATE UNIQUE INDEX IF NOT EXISTS user_auth_password_user_id_uindex ON user_auth_password (user_id);
