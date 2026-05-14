/***Statement***/
DELETE FROM admin_users
WHERE rowid NOT IN (SELECT MAX(rowid) FROM admin_users GROUP BY username);
/***Statement***/
DROP INDEX IF EXISTS admin_users_username_index;
/***Statement***/
CREATE UNIQUE INDEX IF NOT EXISTS admin_users_username_unique_idx ON admin_users (username);
