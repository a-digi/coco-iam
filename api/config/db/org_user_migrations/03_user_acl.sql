/***Statement***/
CREATE TABLE IF NOT EXISTS user_acl
(
    id TEXT NOT NULL CONSTRAINT user_acl_pk PRIMARY KEY UNIQUE,
    user_id TEXT NOT NULL,
    roles JSON NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    is_active BOOLEAN NOT NULL DEFAULT TRUE
);
/***Statement***/
CREATE UNIQUE INDEX IF NOT EXISTS user_acl_user_id_uindex ON user_acl (user_id);
/***Statement***/
CREATE INDEX IF NOT EXISTS user_acl_id_index ON user_acl (id);
/***Statement***/
CREATE INDEX IF NOT EXISTS user_acl_is_active_index ON user_acl (user_id);
