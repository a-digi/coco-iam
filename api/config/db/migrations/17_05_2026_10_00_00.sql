/***Statement***/
CREATE TABLE IF NOT EXISTS app_settings (
    id TEXT NOT NULL CONSTRAINT app_settings_pk PRIMARY KEY UNIQUE,
    key TEXT NOT NULL,
    value TEXT NOT NULL DEFAULT '',
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
/***Statement***/
CREATE UNIQUE INDEX IF NOT EXISTS app_settings_key_uindex ON app_settings (key);
