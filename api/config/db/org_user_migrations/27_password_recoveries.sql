/***Statement***/
CREATE TABLE IF NOT EXISTS password_recoveries (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at DATETIME NOT NULL,
    consumed_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
/***Statement***/
CREATE UNIQUE INDEX IF NOT EXISTS password_recoveries_token_hash_uindex ON password_recoveries (token_hash);
/***Statement***/
CREATE INDEX IF NOT EXISTS password_recoveries_user_idx ON password_recoveries (user_id);
/***Statement***/
CREATE INDEX IF NOT EXISTS password_recoveries_expires_idx ON password_recoveries (expires_at);
