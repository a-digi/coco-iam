/***Statement***/
-- No-op: the `resource_ids` column on application_scopes is now
-- declared directly in the baseline migration
-- (15_02_2026_12_21_22.sql). Re-running ALTER TABLE ADD COLUMN here
-- would fail with "duplicate column name" on a fresh DB. Kept as a
-- placeholder so existing DBs that already tracked this filename
-- keep a 1:1 mapping against the migrations folder.
SELECT 1;
