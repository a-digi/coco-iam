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
