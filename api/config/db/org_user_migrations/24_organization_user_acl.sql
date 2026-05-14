/***Statement***/
CREATE TABLE IF NOT EXISTS organization_user_acl (
    id TEXT NOT NULL CONSTRAINT organization_user_acl_pk PRIMARY KEY UNIQUE,
    user_id TEXT NOT NULL,
    roles JSON NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    is_active BOOLEAN NOT NULL DEFAULT TRUE
);
/***Statement***/
CREATE UNIQUE INDEX IF NOT EXISTS organization_user_acl_user_id_uindex ON organization_user_acl (user_id);
/***Statement***/
CREATE INDEX IF NOT EXISTS organization_user_acl_id_index ON organization_user_acl (id);
