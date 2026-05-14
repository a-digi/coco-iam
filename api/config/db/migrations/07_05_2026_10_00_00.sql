/***Statement***/
CREATE TABLE IF NOT EXISTS admin_acl (
    id TEXT NOT NULL CONSTRAINT admin_acl_pk PRIMARY KEY UNIQUE,
    user_id TEXT NOT NULL,
    roles JSON NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    is_active BOOLEAN NOT NULL DEFAULT TRUE
);
/***Statement***/
CREATE UNIQUE INDEX IF NOT EXISTS admin_acl_user_id_uindex ON admin_acl (user_id);
/***Statement***/
CREATE INDEX IF NOT EXISTS admin_acl_id_index ON admin_acl (id);
/***Statement***/
CREATE INDEX IF NOT EXISTS admin_acl_is_active_index ON admin_acl (user_id);
/***Statement***/
INSERT OR IGNORE INTO admin_acl (id, user_id, roles, created_at, is_active)
SELECT id, user_id, roles, created_at, is_active FROM user_acl
