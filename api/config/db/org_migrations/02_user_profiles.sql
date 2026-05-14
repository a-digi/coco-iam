/***Statement***/
CREATE TABLE IF NOT EXISTS user_profiles
(
    user_id TEXT NOT NULL CONSTRAINT user_profiles_pk PRIMARY KEY UNIQUE,
    profile_data TEXT NOT NULL DEFAULT '{}',
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
