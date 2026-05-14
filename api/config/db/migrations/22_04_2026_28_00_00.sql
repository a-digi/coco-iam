/***Statement***/
-- Side-panel text is now stored per-column in
-- application_login_columns (text_block_title / text_block_body).
-- The wrapper-level fallback columns are no longer used anywhere in
-- the render path and are removed here.
ALTER TABLE application_login_settings DROP COLUMN text_block_title;
/***Statement***/
ALTER TABLE application_login_settings DROP COLUMN text_block_body;
