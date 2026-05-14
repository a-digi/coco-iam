/***Statement***/
-- Each text column has a single title + a list of content blocks.
-- Replace the previous `text_blocks` JSON (which embedded both
-- title and body per block) with two separate columns:
--   text_block_title  TEXT nullable   — one title per column
--   text_contents     TEXT NOT NULL   — JSON array [{id,content}]
ALTER TABLE application_login_columns DROP COLUMN text_blocks;
/***Statement***/
ALTER TABLE application_login_columns ADD COLUMN text_block_title TEXT;
/***Statement***/
ALTER TABLE application_login_columns ADD COLUMN text_contents TEXT NOT NULL DEFAULT '[]';
