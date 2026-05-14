/***Statement***/
-- Per-column text is now a JSON list of content blocks instead of a
-- single title + body. Each block is
-- {"id":"...","title":"...","body":"..."}. Removing the old singular
-- columns keeps the schema honest with the new UI.
ALTER TABLE application_login_columns ADD COLUMN text_blocks TEXT NOT NULL DEFAULT '[]';
/***Statement***/
ALTER TABLE application_login_columns DROP COLUMN text_block_title;
/***Statement***/
ALTER TABLE application_login_columns DROP COLUMN text_block_body;
