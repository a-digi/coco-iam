/***Statement***/
CREATE TABLE IF NOT EXISTS admin_password_notify_prefs (
    user_id TEXT NOT NULL CONSTRAINT admin_password_notify_prefs_pk PRIMARY KEY UNIQUE,
    notify_days TEXT NOT NULL DEFAULT '[]',
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
/***Statement***/
CREATE TABLE IF NOT EXISTS admin_password_notify_log (
    id TEXT NOT NULL CONSTRAINT admin_password_notify_log_pk PRIMARY KEY UNIQUE,
    user_id TEXT NOT NULL,
    password_changed_at TEXT NOT NULL,
    days_before INTEGER NOT NULL,
    sent_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
/***Statement***/
CREATE UNIQUE INDEX IF NOT EXISTS admin_password_notify_log_uindex ON admin_password_notify_log (user_id, password_changed_at, days_before);
/***Statement***/
CREATE INDEX IF NOT EXISTS admin_password_notify_log_user_idx ON admin_password_notify_log (user_id);
