/***Statement***/
CREATE TABLE IF NOT EXISTS application_login_columns (
    application_id TEXT NOT NULL,
    column_index INTEGER NOT NULL,
    background_color TEXT,
    background_asset_id TEXT,
    background_gradient_from TEXT,
    background_gradient_to TEXT,
    background_gradient_angle INTEGER,
    text_color_override TEXT,
    text_block_title TEXT,
    text_contents TEXT NOT NULL DEFAULT '[]',
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (application_id, column_index)
);
/***Statement***/
CREATE INDEX IF NOT EXISTS application_login_columns_app_idx
    ON application_login_columns (application_id);
