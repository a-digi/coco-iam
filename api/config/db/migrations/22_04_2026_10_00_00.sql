/***Statement***/
-- Self-healing CREATE so this migration runs cleanly on data
-- dirs where an older 15_02 was recorded as complete BEFORE
-- it carried the application_login_settings CREATE block.
-- Fresh installs already have the table from 15_02, so the
-- IF NOT EXISTS makes this a no-op for them.
CREATE TABLE IF NOT EXISTS application_login_settings (
    application_id  TEXT PRIMARY KEY,
    redirect_url    TEXT NOT NULL DEFAULT '',
    redirect_method TEXT NOT NULL DEFAULT 'POST',
    redirect_secret TEXT NOT NULL DEFAULT '',
    custom_headers  TEXT NOT NULL DEFAULT '{}',
    updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);
/***Statement***/
-- Same defensive CREATE for application_login_assets — the
-- last ALTER in this file lands on it, so the table must
-- exist regardless of whether 15_02 actually created it.
CREATE TABLE IF NOT EXISTS application_login_assets (
    id             TEXT PRIMARY KEY,
    application_id TEXT NOT NULL,
    file_path      TEXT NOT NULL,
    mime_type      TEXT NOT NULL,
    size_bytes     INTEGER NOT NULL,
    created_at     DATETIME DEFAULT CURRENT_TIMESTAMP
);
/***Statement***/
DROP TABLE IF EXISTS application_login_templates;
/***Statement***/
DROP TABLE IF EXISTS application_recovery_templates;
/***Statement***/
ALTER TABLE application_login_settings ADD COLUMN template_kind       TEXT    NOT NULL DEFAULT 'centered_1col';
/***Statement***/
ALTER TABLE application_login_settings ADD COLUMN background_color    TEXT    NOT NULL DEFAULT '#f9fafb';
/***Statement***/
ALTER TABLE application_login_settings ADD COLUMN background_asset_id TEXT;
/***Statement***/
ALTER TABLE application_login_settings ADD COLUMN logo_asset_id       TEXT;
/***Statement***/
ALTER TABLE application_login_settings ADD COLUMN show_logo           BOOLEAN NOT NULL DEFAULT 1;
/***Statement***/
ALTER TABLE application_login_settings ADD COLUMN page_title          TEXT    NOT NULL DEFAULT '';
/***Statement***/
ALTER TABLE application_login_settings ADD COLUMN brand_text          TEXT    NOT NULL DEFAULT '';
/***Statement***/
ALTER TABLE application_login_settings ADD COLUMN text_block_title    TEXT    NOT NULL DEFAULT '';
/***Statement***/
ALTER TABLE application_login_settings ADD COLUMN text_block_body     TEXT    NOT NULL DEFAULT '';
/***Statement***/
ALTER TABLE application_login_settings ADD COLUMN allow_recovery      BOOLEAN NOT NULL DEFAULT 1;
/***Statement***/
ALTER TABLE application_login_settings ADD COLUMN allow_registration  BOOLEAN NOT NULL DEFAULT 0;
/***Statement***/
ALTER TABLE application_login_assets ADD COLUMN kind TEXT NOT NULL DEFAULT 'other';
