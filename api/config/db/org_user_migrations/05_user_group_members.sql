/***Statement***/
-- group_id is a loose reference to the main-DB `user_groups` table.
-- See the note on application_user_acl: consistency at the app layer.
CREATE TABLE IF NOT EXISTS user_group_members
(
    id TEXT NOT NULL CONSTRAINT user_group_members_pk PRIMARY KEY UNIQUE,
    user_id TEXT NOT NULL,
    group_id TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    is_active BOOLEAN NOT NULL DEFAULT TRUE
);
/***Statement***/
CREATE UNIQUE INDEX IF NOT EXISTS user_group_members_user_id_group_id_uindex
    ON user_group_members (user_id, group_id);
/***Statement***/
CREATE INDEX IF NOT EXISTS user_group_members_id_index ON user_group_members (id);
/***Statement***/
CREATE INDEX IF NOT EXISTS user_group_members_group_id_index ON user_group_members (group_id);
/***Statement***/
CREATE INDEX IF NOT EXISTS user_group_members_user_id_index ON user_group_members (user_id);
