/***Statement***/
CREATE TABLE IF NOT EXISTS admin_auth_password (
    user_id TEXT NOT NULL CONSTRAINT admin_auth_password_pk PRIMARY KEY UNIQUE,
    password TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    is_active BOOLEAN NOT NULL DEFAULT TRUE
);
/***Statement***/
CREATE UNIQUE INDEX IF NOT EXISTS admin_auth_password_user_id_uindex
    ON admin_auth_password (user_id);
/***Statement***/
INSERT OR IGNORE INTO admin_auth_password (user_id, password, created_at, is_active)
SELECT p.user_id, p.password, p.created_at, p.is_active
FROM user_auth_password p
WHERE p.user_id IN (SELECT id FROM admin_users);
/***Statement***/
DROP TABLE IF EXISTS user_auth_password;
