/***Statement***/
CREATE TABLE IF NOT EXISTS admin_groups (
    id TEXT NOT NULL CONSTRAINT admin_groups_pk PRIMARY KEY UNIQUE,
    title TEXT NOT NULL,
    group_description TEXT NOT NULL DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    is_active BOOLEAN NOT NULL DEFAULT TRUE
);
/***Statement***/
CREATE UNIQUE INDEX IF NOT EXISTS admin_groups_id_uindex ON admin_groups (id);
/***Statement***/
CREATE INDEX IF NOT EXISTS admin_groups_title_index ON admin_groups (title);
/***Statement***/
CREATE TABLE IF NOT EXISTS admin_group_members (
    id TEXT NOT NULL CONSTRAINT admin_group_members_pk PRIMARY KEY UNIQUE,
    user_id TEXT NOT NULL,
    group_id TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    is_active BOOLEAN NOT NULL DEFAULT TRUE
);
/***Statement***/
CREATE UNIQUE INDEX IF NOT EXISTS admin_group_members_user_group_uindex ON admin_group_members (user_id, group_id);
/***Statement***/
CREATE INDEX IF NOT EXISTS admin_group_members_group_id_index ON admin_group_members (group_id);
/***Statement***/
CREATE INDEX IF NOT EXISTS admin_group_members_user_id_index ON admin_group_members (user_id);
/***Statement***/
CREATE TABLE IF NOT EXISTS admin_group_acl (
    id TEXT NOT NULL CONSTRAINT admin_group_acl_pk PRIMARY KEY UNIQUE,
    group_id TEXT NOT NULL,
    roles JSON NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    is_active BOOLEAN NOT NULL DEFAULT TRUE
);
/***Statement***/
CREATE UNIQUE INDEX IF NOT EXISTS admin_group_acl_group_id_uindex ON admin_group_acl (group_id);
/***Statement***/
CREATE INDEX IF NOT EXISTS admin_group_acl_group_id_index ON admin_group_acl (group_id);
/***Statement***/
CREATE TABLE IF NOT EXISTS user_groups (
    id TEXT NOT NULL CONSTRAINT user_groups_pk PRIMARY KEY UNIQUE,
    group_type TEXT NOT NULL,
    title TEXT NOT NULL,
    group_description TEXT NOT NULL DEFAULT '',
    organization_id TEXT NOT NULL DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    is_active BOOLEAN NOT NULL DEFAULT TRUE
);
/***Statement***/
CREATE TABLE IF NOT EXISTS user_group_members (
    id TEXT NOT NULL CONSTRAINT user_group_members_pk PRIMARY KEY UNIQUE,
    user_id TEXT NOT NULL,
    group_id TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    is_active BOOLEAN NOT NULL DEFAULT TRUE
);
/***Statement***/
CREATE TABLE IF NOT EXISTS user_group_acl (
    id TEXT NOT NULL CONSTRAINT user_group_acl_pk PRIMARY KEY UNIQUE,
    group_id TEXT NOT NULL,
    roles JSON NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    is_active BOOLEAN NOT NULL DEFAULT TRUE
);
/***Statement***/
INSERT OR IGNORE INTO admin_groups (id, title, group_description, created_at, is_active)
SELECT id, title, group_description, created_at, is_active
FROM user_groups WHERE group_type = 'admin';
/***Statement***/
INSERT OR IGNORE INTO admin_group_members (id, user_id, group_id, created_at, is_active)
SELECT id, user_id, group_id, created_at, is_active
FROM user_group_members
WHERE group_id IN (SELECT id FROM user_groups WHERE group_type = 'admin');
/***Statement***/
INSERT OR IGNORE INTO admin_group_acl (id, group_id, roles, created_at, is_active)
SELECT id, group_id, roles, created_at, is_active
FROM user_group_acl
WHERE group_id IN (SELECT id FROM user_groups WHERE group_type = 'admin');
/***Statement***/
DELETE FROM user_group_members
WHERE group_id IN (SELECT id FROM user_groups WHERE group_type = 'admin');
/***Statement***/
DELETE FROM user_group_acl
WHERE group_id IN (SELECT id FROM user_groups WHERE group_type = 'admin');
/***Statement***/
DELETE FROM user_groups WHERE group_type = 'admin';
