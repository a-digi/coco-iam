/***Statement***/
-- Per-column text content for split login templates. Nullable so an
-- empty field inherits the wrapper-level text_block_title /
-- text_block_body (backward compatible with templates saved before
-- this migration).
ALTER TABLE application_login_columns ADD COLUMN text_block_title TEXT;
/***Statement***/
ALTER TABLE application_login_columns ADD COLUMN text_block_body TEXT;
