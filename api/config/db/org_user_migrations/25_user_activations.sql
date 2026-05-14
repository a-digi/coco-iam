/***Statement***/
CREATE TABLE IF NOT EXISTS user_activations (
    id TEXT NOT NULL CONSTRAINT user_activations_pk PRIMARY KEY UNIQUE,
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
CREATE UNIQUE INDEX IF NOT EXISTS user_activations_token_hash_uindex ON user_activations (token_hash);
/***Statement***/
CREATE INDEX IF NOT EXISTS user_activations_user_idx ON user_activations (user_id);
/***Statement***/
CREATE INDEX IF NOT EXISTS user_activations_expires_idx ON user_activations (expires_at);
