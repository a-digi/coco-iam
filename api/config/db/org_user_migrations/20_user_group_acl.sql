/***Statement***/
CREATE TABLE IF NOT EXISTS user_group_acl (
    id TEXT NOT NULL CONSTRAINT user_group_acl_pk PRIMARY KEY UNIQUE,
    group_id TEXT NOT NULL,
    roles JSON NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    is_active BOOLEAN NOT NULL DEFAULT TRUE
);
/***Statement***/
CREATE UNIQUE INDEX IF NOT EXISTS user_group_acl_group_id_uindex ON user_group_acl (group_id);
/***Statement***/
CREATE INDEX IF NOT EXISTS user_group_acl_id_index ON user_group_acl (id);
