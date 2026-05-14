/***Statement***/
CREATE TABLE IF NOT EXISTS organization
(
    id text not null constraint organization_pk primary key constraint organization_pk_2 unique,
    title text not null,
    description text NOT NULL DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    is_active BOOLEAN NOT NULL DEFAULT TRUE
);
/***Statement***/
CREATE TABLE IF NOT EXISTS workspace
(
    id TEXT NOT NULL CONSTRAINT workspace_pk PRIMARY KEY,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    organization_id TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    is_active BOOLEAN NOT NULL DEFAULT TRUE
);
/***Statement***/
CREATE INDEX IF NOT EXISTS workspace_id_index ON workspace (id);
/***Statement***/
CREATE INDEX IF NOT EXISTS workspace_organization_id_idx ON workspace (organization_id);
/***Statement***/
CREATE TABLE IF NOT EXISTS users
(
    id TEXT NOT NULL CONSTRAINT users_pk PRIMARY KEY UNIQUE,
    username TEXT NOT NULL,
    email TEXT NOT NULL,
    organization_id TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    must_change_password BOOLEAN NOT NULL DEFAULT FALSE
);
/***Statement***/
CREATE INDEX IF NOT EXISTS users_email_index ON users (email);
/***Statement***/
CREATE INDEX IF NOT EXISTS users_id_index ON users (id);
/***Statement***/
CREATE INDEX IF NOT EXISTS users_is_active_index ON users (is_active);
/***Statement***/
CREATE INDEX IF NOT EXISTS users_username_index ON users (username);
/***Statement***/
CREATE TABLE IF NOT EXISTS user_auth_password
(
    user_id TEXT NOT NULL CONSTRAINT user_auth_password_pk PRIMARY KEY CONSTRAINT user_auth_password_pk_2 UNIQUE,
    password TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    is_active BOOLEAN NOT NULL DEFAULT TRUE
);
/***Statement***/
CREATE UNIQUE INDEX IF NOT EXISTS user_auth_password_user_id_uindex ON user_auth_password (user_id);
/***Statement***/
CREATE TABLE IF NOT EXISTS admin_users
(
    id TEXT NOT NULL CONSTRAINT admin_users_pk PRIMARY KEY UNIQUE,
    username TEXT NOT NULL,
    email TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    limited_access BOOLEAN NOT NULL DEFAULT TRUE,
    is_super_admin BOOLEAN NOT NULL DEFAULT FALSE,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    must_change_password BOOLEAN NOT NULL DEFAULT FALSE
);
/***Statement***/
CREATE INDEX IF NOT EXISTS admin_users_email_index ON admin_users (email);
/***Statement***/
CREATE INDEX IF NOT EXISTS admin_users_id_index ON admin_users (id);
/***Statement***/
CREATE INDEX IF NOT EXISTS admin_users_is_active_index ON admin_users (is_active);
/***Statement***/
CREATE INDEX IF NOT EXISTS admin_users_username_index ON admin_users (username);
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
/***Statement***/
CREATE TABLE IF NOT EXISTS user_groups
(
    id TEXT NOT NULL CONSTRAINT user_groups_pk PRIMARY KEY UNIQUE,
    group_type TEXT NOT NULL,
    title TEXT NOT NULL,
    group_description TEXT NOT NULL DEFAULT '',
    organization_id TEXT NOT NULL DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    is_active BOOLEAN NOT NULL DEFAULT TRUE
);
/***Statement***/
CREATE UNIQUE INDEX IF NOT EXISTS user_groups_user_id_uindex ON user_groups (id);
/***Statement***/
CREATE INDEX IF NOT EXISTS user_groups_id_index ON user_groups (id);
/***Statement***/
CREATE INDEX IF NOT EXISTS user_groups_is_active_index ON user_groups (title);
/***Statement***/
CREATE INDEX IF NOT EXISTS user_groups_organization_id_index ON user_groups (organization_id);
/***Statement***/
CREATE TABLE IF NOT EXISTS user_group_members
(
    id TEXT NOT NULL CONSTRAINT user_group_members_pk PRIMARY KEY UNIQUE,
    user_id TEXT NOT NULL,
    group_id TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    is_active BOOLEAN NOT NULL DEFAULT TRUE
);
/***Statement***/
CREATE UNIQUE INDEX IF NOT EXISTS user_group_members_user_id_group_id_uindex ON user_group_members (user_id, group_id);
/***Statement***/
CREATE INDEX IF NOT EXISTS user_group_members_id_index ON user_group_members (id);
/***Statement***/
CREATE INDEX IF NOT EXISTS user_group_members_group_id_index ON user_group_members (group_id);
/***Statement***/
CREATE INDEX IF NOT EXISTS user_group_members_user_id_index ON user_group_members (user_id);
/***Statement***/
CREATE INDEX IF NOT EXISTS user_group_members_user_id_group_id_index ON user_group_members (user_id, group_id);
/***Statement***/
CREATE TABLE IF NOT EXISTS user_group_acl
(
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
/***Statement***/
CREATE INDEX IF NOT EXISTS user_group_acl_is_active_index ON user_group_acl (group_id);
/***Statement***/
CREATE TABLE IF NOT EXISTS organization_user_acl
(
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
/***Statement***/
CREATE TABLE IF NOT EXISTS organization_group_acl
(
    id TEXT NOT NULL CONSTRAINT organization_group_acl_pk PRIMARY KEY UNIQUE,
    group_id TEXT NOT NULL,
    roles JSON NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    is_active BOOLEAN NOT NULL DEFAULT TRUE
);
/***Statement***/
CREATE UNIQUE INDEX IF NOT EXISTS organization_group_acl_group_id_uindex ON organization_group_acl (group_id);
/***Statement***/
CREATE INDEX IF NOT EXISTS organization_group_acl_id_index ON organization_group_acl (id);
/***Statement***/
CREATE TABLE IF NOT EXISTS applications
(
    id TEXT NOT NULL CONSTRAINT applications_pk PRIMARY KEY UNIQUE,
    workspace_id TEXT NOT NULL,
    client_id TEXT NOT NULL,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    is_active BOOLEAN NOT NULL DEFAULT TRUE
);
/***Statement***/
CREATE UNIQUE INDEX IF NOT EXISTS applications_client_id_uindex ON applications (client_id);
/***Statement***/
CREATE INDEX IF NOT EXISTS applications_workspace_id_index ON applications (workspace_id);
/***Statement***/
CREATE INDEX IF NOT EXISTS applications_id_index ON applications (id);
/***Statement***/
CREATE TABLE IF NOT EXISTS application_scopes
(
    id TEXT NOT NULL CONSTRAINT application_scopes_pk PRIMARY KEY UNIQUE,
    application_id TEXT NOT NULL,
    scope_id TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    resource_ids TEXT NOT NULL DEFAULT '[]',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    is_active BOOLEAN NOT NULL DEFAULT TRUE
);
/***Statement***/
CREATE UNIQUE INDEX IF NOT EXISTS application_scopes_application_id_scope_id_uindex ON application_scopes (application_id, scope_id);
/***Statement***/
CREATE INDEX IF NOT EXISTS application_scopes_application_id_index ON application_scopes (application_id);
/***Statement***/
CREATE TABLE IF NOT EXISTS application_user_acl
(
    id TEXT NOT NULL CONSTRAINT application_user_acl_pk PRIMARY KEY UNIQUE,
    application_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    roles JSON NOT NULL,
    grantable_roles TEXT NOT NULL DEFAULT '[]',
    resource_ids TEXT NOT NULL DEFAULT '{}',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    is_active BOOLEAN NOT NULL DEFAULT TRUE
);
/***Statement***/
CREATE UNIQUE INDEX IF NOT EXISTS application_user_acl_application_id_user_id_uindex ON application_user_acl (application_id, user_id);
/***Statement***/
CREATE INDEX IF NOT EXISTS application_user_acl_application_id_index ON application_user_acl (application_id);
/***Statement***/
CREATE INDEX IF NOT EXISTS application_user_acl_user_id_index ON application_user_acl (user_id);
/***Statement***/
CREATE TABLE IF NOT EXISTS application_group_acl
(
    id TEXT NOT NULL CONSTRAINT application_group_acl_pk PRIMARY KEY UNIQUE,
    application_id TEXT NOT NULL,
    group_id TEXT NOT NULL,
    roles JSON NOT NULL,
    grantable_roles TEXT NOT NULL DEFAULT '[]',
    resource_ids TEXT NOT NULL DEFAULT '{}',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    is_active BOOLEAN NOT NULL DEFAULT TRUE
);
/***Statement***/
CREATE UNIQUE INDEX IF NOT EXISTS application_group_acl_application_id_group_id_uindex ON application_group_acl (application_id, group_id);
/***Statement***/
CREATE INDEX IF NOT EXISTS application_group_acl_application_id_idx ON application_group_acl (application_id);
/***Statement***/
CREATE INDEX IF NOT EXISTS application_group_acl_group_id_idx ON application_group_acl (group_id);
/***Statement***/
CREATE TABLE IF NOT EXISTS application_login_templates (
    application_id TEXT PRIMARY KEY,
    body_html      TEXT NOT NULL DEFAULT '',
    updated_at     DATETIME DEFAULT CURRENT_TIMESTAMP
);
/***Statement***/
CREATE TABLE IF NOT EXISTS application_recovery_templates (
    application_id    TEXT PRIMARY KEY,
    request_body_html TEXT NOT NULL DEFAULT '',
    reset_body_html   TEXT NOT NULL DEFAULT '',
    updated_at        DATETIME DEFAULT CURRENT_TIMESTAMP
);
/***Statement***/
CREATE TABLE IF NOT EXISTS application_login_assets (
    id             TEXT PRIMARY KEY,
    application_id TEXT NOT NULL,
    file_path      TEXT NOT NULL,
    mime_type      TEXT NOT NULL,
    size_bytes     INTEGER NOT NULL,
    created_at     DATETIME DEFAULT CURRENT_TIMESTAMP
);
/***Statement***/
CREATE INDEX IF NOT EXISTS application_login_assets_app_idx ON application_login_assets(application_id);
/***Statement***/
CREATE TABLE IF NOT EXISTS application_login_settings (
    application_id  TEXT PRIMARY KEY,
    redirect_url    TEXT NOT NULL DEFAULT '',
    redirect_method TEXT NOT NULL DEFAULT 'POST',
    redirect_secret TEXT NOT NULL DEFAULT '',
    custom_headers  TEXT NOT NULL DEFAULT '{}',
    updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);
/***Statement***/
CREATE TABLE IF NOT EXISTS application_keys (
    id              TEXT PRIMARY KEY,
    application_id  TEXT NOT NULL,
    status          TEXT NOT NULL,
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    activated_at    DATETIME,
    deactivated_at  DATETIME,
    expires_at      DATETIME
);
/***Statement***/
CREATE INDEX IF NOT EXISTS application_keys_app_idx ON application_keys(application_id);
/***Statement***/
CREATE INDEX IF NOT EXISTS application_keys_app_status_idx ON application_keys(application_id, status);
/***Statement***/
CREATE TABLE IF NOT EXISTS queues
(
    id TEXT NOT NULL CONSTRAINT queues_pk PRIMARY KEY UNIQUE,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
/***Statement***/
CREATE UNIQUE INDEX IF NOT EXISTS queues_name_uindex ON queues (name);
/***Statement***/
CREATE INDEX IF NOT EXISTS queues_id_idx ON queues (id);
/***Statement***/
CREATE TABLE IF NOT EXISTS queue_tasks
(
    id TEXT NOT NULL CONSTRAINT queue_tasks_pk PRIMARY KEY UNIQUE,
    queue_name TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    attempts INTEGER NOT NULL DEFAULT 0,
    max_attempts INTEGER NOT NULL DEFAULT 3,
    last_error TEXT NOT NULL DEFAULT '',
    next_attempt_at TEXT NOT NULL DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    completed_at TEXT NOT NULL DEFAULT ''
);
/***Statement***/
CREATE INDEX IF NOT EXISTS queue_tasks_queue_name_status_idx ON queue_tasks (queue_name, status);
/***Statement***/
CREATE INDEX IF NOT EXISTS queue_tasks_status_idx ON queue_tasks (status);
/***Statement***/
CREATE INDEX IF NOT EXISTS queue_tasks_next_attempt_at_idx ON queue_tasks (next_attempt_at);
/***Statement***/
CREATE INDEX IF NOT EXISTS queue_tasks_created_at_idx ON queue_tasks (created_at);
/***Statement***/
CREATE TABLE IF NOT EXISTS user_activations
(
    id TEXT NOT NULL CONSTRAINT user_activations_pk PRIMARY KEY UNIQUE,
    user_id TEXT NOT NULL,
    user_type TEXT NOT NULL,
    token_hash TEXT NOT NULL,
    temp_password_hash TEXT NOT NULL,
    expires_at DATETIME NOT NULL,
    consumed_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
/***Statement***/
CREATE UNIQUE INDEX IF NOT EXISTS user_activations_token_hash_uindex ON user_activations (token_hash);
/***Statement***/
CREATE INDEX IF NOT EXISTS user_activations_user_idx ON user_activations (user_id, user_type);
/***Statement***/
CREATE INDEX IF NOT EXISTS user_activations_expires_idx ON user_activations (expires_at);
/***Statement***/
CREATE TABLE IF NOT EXISTS app_settings
(
    id TEXT NOT NULL CONSTRAINT app_settings_pk PRIMARY KEY UNIQUE,
    key TEXT NOT NULL UNIQUE,
    value TEXT NOT NULL DEFAULT '',
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
/***Statement***/
CREATE UNIQUE INDEX IF NOT EXISTS app_settings_key_uindex ON app_settings (key);
/***Statement***/
CREATE TABLE IF NOT EXISTS user_rule_sets (
    id TEXT PRIMARY KEY,
    scope TEXT NOT NULL,
    owner_id TEXT NOT NULL,
    rules_json TEXT NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (scope, owner_id)
);
/***Statement***/
CREATE INDEX IF NOT EXISTS user_rule_sets_scope_idx ON user_rule_sets (scope, owner_id);
/***Statement***/
CREATE TABLE IF NOT EXISTS password_recoveries (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    user_type TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at DATETIME NOT NULL,
    consumed_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
/***Statement***/
CREATE INDEX IF NOT EXISTS password_recoveries_user_idx ON password_recoveries (user_id, user_type);
/***Statement***/
CREATE INDEX IF NOT EXISTS password_recoveries_expires_idx ON password_recoveries (expires_at);
/***Statement***/
CREATE TABLE IF NOT EXISTS media_folders (
    id         TEXT PRIMARY KEY,
    owner_id   TEXT NOT NULL,
    parent_id  TEXT,
    slug       TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (owner_id, parent_id, slug)
);
/***Statement***/
CREATE INDEX IF NOT EXISTS media_folders_owner_parent_idx ON media_folders(owner_id, parent_id);
/***Statement***/
CREATE TABLE IF NOT EXISTS media_files (
    id           TEXT PRIMARY KEY,
    owner_id     TEXT NOT NULL,
    folder_id    TEXT,
    filename     TEXT NOT NULL,
    mime_type    TEXT NOT NULL,
    size_bytes   INTEGER NOT NULL,
    on_disk_path TEXT NOT NULL,
    created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (owner_id, folder_id, filename)
);
/***Statement***/
CREATE INDEX IF NOT EXISTS media_files_owner_folder_idx ON media_files(owner_id, folder_id);
/***Statement***/
INSERT OR IGNORE INTO organization (id, title, description, is_active)
VALUES ('00000000-0000-0000-0000-000000000001', 'Default Organization', 'Auto-seeded default organization', TRUE);
