/***Statement***/
CREATE TABLE IF NOT EXISTS application_login_assets (
    id TEXT PRIMARY KEY,
    application_id TEXT NOT NULL,
    file_path TEXT NOT NULL,
    mime_type TEXT NOT NULL,
    size_bytes INTEGER NOT NULL,
    kind TEXT NOT NULL DEFAULT 'other',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
/***Statement***/
CREATE INDEX IF NOT EXISTS application_login_assets_app_idx ON application_login_assets(application_id);
