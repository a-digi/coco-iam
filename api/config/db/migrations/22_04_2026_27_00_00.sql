/***Statement***/
-- Rich-text editor defaults per login template. Stores the last
-- colour / font-size / margin the admin picked in the WYSIWYG
-- toolbar so the editor can reopen in the same state and not
-- surprise the admin with a reset swatch. JSON shape:
--   { "foreground_color":"#ff0000", "font_size":"1rem", "block_margin":"0" }
-- Every field is optional.
ALTER TABLE application_login_settings
    ADD COLUMN rich_text_defaults TEXT NOT NULL DEFAULT '{}';
