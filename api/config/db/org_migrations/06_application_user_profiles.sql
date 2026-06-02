/***Statement***/
CREATE TABLE IF NOT EXISTS application_user_profiles
(
    id             TEXT     NOT NULL PRIMARY KEY UNIQUE,
    application_id TEXT     NOT NULL,
    user_id        TEXT     NOT NULL,
    profile_data   TEXT     NOT NULL DEFAULT '{}',
    updated_at     DATETIME DEFAULT CURRENT_TIMESTAMP
);
/***Statement***/
CREATE UNIQUE INDEX IF NOT EXISTS application_user_profiles_app_user_idx
    ON application_user_profiles (application_id, user_id);
