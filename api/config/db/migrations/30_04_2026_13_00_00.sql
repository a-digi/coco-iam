/***Statement***/
CREATE TABLE IF NOT EXISTS admin_user_profiles
(
    admin_user_id   TEXT NOT NULL CONSTRAINT admin_user_profiles_pk PRIMARY KEY UNIQUE,
    first_name      TEXT NOT NULL DEFAULT '',
    last_name       TEXT NOT NULL DEFAULT '',
    phone           TEXT NOT NULL DEFAULT '',
    avatar_asset_id TEXT NOT NULL DEFAULT '',
    locale          TEXT NOT NULL DEFAULT '',
    timezone        TEXT NOT NULL DEFAULT '',
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);
