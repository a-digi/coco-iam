/***Statement***/
CREATE TABLE IF NOT EXISTS application_login_settings (
    application_id  TEXT PRIMARY KEY,
    redirect_url    TEXT NOT NULL DEFAULT '',
    redirect_method TEXT NOT NULL DEFAULT 'POST',
    redirect_secret TEXT NOT NULL DEFAULT '',
    custom_headers  TEXT NOT NULL DEFAULT '{}',
    updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);
