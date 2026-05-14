/***Statement***/
ALTER TABLE admin_auth_password ADD COLUMN changed_at DATETIME;
/***Statement***/
UPDATE admin_auth_password SET changed_at = created_at WHERE changed_at IS NULL;
