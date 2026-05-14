/***Statement***/
ALTER TABLE user_auth_password ADD COLUMN changed_at DATETIME;
/***Statement***/
UPDATE user_auth_password SET changed_at = created_at WHERE changed_at IS NULL;
