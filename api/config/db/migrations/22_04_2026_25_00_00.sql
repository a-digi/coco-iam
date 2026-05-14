/***Statement***/
-- Per-column login-template backgrounds. Split layouts
-- (split_login_left / split_login_right) let admins customise each
-- visual column independently. column_index 0 = left, column_index 1
-- = right — the schema supports higher indices for future
-- multi-column layouts. All per-column fields are nullable. An empty
-- row (or missing row) means "inherit the wrapper-level background".
CREATE TABLE IF NOT EXISTS application_login_columns (
    application_id            TEXT NOT NULL,
    column_index              INTEGER NOT NULL,
    background_color          TEXT,
    background_asset_id       TEXT,
    background_gradient_from  TEXT,
    background_gradient_to    TEXT,
    background_gradient_angle INTEGER,
    text_color_override       TEXT,
    updated_at                DATETIME DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (application_id, column_index)
);
/***Statement***/
CREATE INDEX IF NOT EXISTS application_login_columns_app_idx
    ON application_login_columns (application_id);
