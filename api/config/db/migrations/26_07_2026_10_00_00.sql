/***Statement***/
CREATE TABLE IF NOT EXISTS ip_bans
(
    ip         TEXT NOT NULL CONSTRAINT ip_bans_pk PRIMARY KEY,
    tier       TEXT NOT NULL,
    reason     TEXT NOT NULL,
    banned_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at DATETIME NOT NULL,
    hit_count  INTEGER NOT NULL DEFAULT 1,
    created_by TEXT
);
/***Statement***/
CREATE INDEX IF NOT EXISTS ip_bans_expires_at_idx
    ON ip_bans (expires_at);
/***Statement***/
CREATE TABLE IF NOT EXISTS ip_allowlist
(
    ip         TEXT NOT NULL CONSTRAINT ip_allowlist_pk PRIMARY KEY,
    note       TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by TEXT NOT NULL
);
