/***Statement***/
DELETE FROM users
WHERE rowid NOT IN (SELECT MAX(rowid) FROM users GROUP BY LOWER(username));
/***Statement***/
DROP INDEX IF EXISTS users_username_idx;
/***Statement***/
CREATE UNIQUE INDEX IF NOT EXISTS users_username_unique_idx ON users (username);
