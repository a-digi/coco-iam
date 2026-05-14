/***Statement***/
DELETE FROM admin_users
WHERE rowid NOT IN (SELECT MAX(rowid) FROM admin_users GROUP BY email);
/***Statement***/
DROP INDEX IF EXISTS admin_users_email_index;
/***Statement***/
CREATE UNIQUE INDEX IF NOT EXISTS admin_users_email_unique_idx ON admin_users (email);
/***Statement***/
DELETE FROM users
WHERE rowid NOT IN (SELECT MAX(rowid) FROM users GROUP BY email, organization_id);
/***Statement***/
DROP INDEX IF EXISTS users_email_index;
/***Statement***/
CREATE UNIQUE INDEX IF NOT EXISTS users_email_organization_unique_idx ON users (email, organization_id);
