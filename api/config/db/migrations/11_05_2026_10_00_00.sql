/***Statement***/
CREATE TABLE IF NOT EXISTS admin_password_recoveries (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at DATETIME NOT NULL,
    consumed_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
/***Statement***/
CREATE UNIQUE INDEX IF NOT EXISTS admin_password_recoveries_token_hash_uindex ON admin_password_recoveries (token_hash);
/***Statement***/
CREATE INDEX IF NOT EXISTS admin_password_recoveries_user_idx ON admin_password_recoveries (user_id);
/***Statement***/
CREATE INDEX IF NOT EXISTS admin_password_recoveries_expires_idx ON admin_password_recoveries (expires_at);
/***Statement***/
INSERT OR IGNORE INTO admin_password_recoveries (id, user_id, token_hash, expires_at, consumed_at, created_at)
SELECT id, user_id, token_hash, expires_at, consumed_at, created_at
FROM password_recoveries WHERE user_type = 'admin';
