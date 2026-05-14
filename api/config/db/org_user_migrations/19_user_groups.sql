/***Statement***/
CREATE TABLE IF NOT EXISTS user_groups (
    id TEXT NOT NULL CONSTRAINT user_groups_pk PRIMARY KEY UNIQUE,
    title TEXT NOT NULL,
    group_description TEXT NOT NULL DEFAULT '',
    organization_id TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    is_active BOOLEAN NOT NULL DEFAULT TRUE
);
/***Statement***/
CREATE INDEX IF NOT EXISTS user_groups_organization_id_index ON user_groups (organization_id);
/***Statement***/
CREATE INDEX IF NOT EXISTS user_groups_title_index ON user_groups (title);
