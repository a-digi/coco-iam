/***Statement***/
CREATE TABLE IF NOT EXISTS application_login_settings (
    application_id TEXT PRIMARY KEY,
    redirect_url TEXT NOT NULL DEFAULT '',
    redirect_method TEXT NOT NULL DEFAULT 'POST',
    redirect_secret TEXT NOT NULL DEFAULT '',
    custom_headers TEXT NOT NULL DEFAULT '{}',
    template_kind TEXT NOT NULL DEFAULT 'centered_1col',
    background_color TEXT NOT NULL DEFAULT '#f9fafb',
    background_asset_id TEXT,
    show_logo BOOLEAN NOT NULL DEFAULT 1,
    page_title TEXT NOT NULL DEFAULT '',
    brand_text TEXT NOT NULL DEFAULT '',
    background_gradient_from TEXT NOT NULL DEFAULT '',
    background_gradient_to TEXT NOT NULL DEFAULT '',
    background_gradient_angle INTEGER NOT NULL DEFAULT 135,
    rich_text_defaults TEXT NOT NULL DEFAULT '{}',
    oauth_client_id TEXT,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
