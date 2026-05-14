/***Statement***/
CREATE TABLE IF NOT EXISTS admin_activations (
    id TEXT NOT NULL CONSTRAINT admin_activations_pk PRIMARY KEY UNIQUE,
    user_id TEXT NOT NULL,
    token_hash TEXT NOT NULL,
    temp_password_hash TEXT NOT NULL,
    expires_at DATETIME NOT NULL,
    consumed_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    redirect_organization_slug TEXT,
    redirect_workspace_slug TEXT,
    redirect_application_client_id TEXT
);
/***Statement***/
CREATE UNIQUE INDEX IF NOT EXISTS admin_activations_token_hash_uindex ON admin_activations (token_hash);
/***Statement***/
CREATE INDEX IF NOT EXISTS admin_activations_user_idx ON admin_activations (user_id);
/***Statement***/
CREATE INDEX IF NOT EXISTS admin_activations_expires_idx ON admin_activations (expires_at);
/***Statement***/
INSERT OR IGNORE INTO admin_activations (id, user_id, token_hash, temp_password_hash, expires_at, consumed_at, created_at, redirect_organization_slug, redirect_workspace_slug, redirect_application_client_id)
SELECT id, user_id, token_hash, temp_password_hash, expires_at, consumed_at, created_at, redirect_organization_slug, redirect_workspace_slug, redirect_application_client_id
FROM user_activations WHERE user_type = 'admin'
