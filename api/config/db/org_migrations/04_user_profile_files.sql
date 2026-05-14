/***Statement***/
-- Two new columns on profile_fields power per-field file-upload
-- policy for DataTypeFile. accept_mime is a comma-separated
-- whitelist that narrows the media subsystem defaults. Empty
-- string means "use the media subsystem default allowlist".
-- max_bytes is the per-field hard size cap. Zero means "use the
-- media default cap for the sniffed mime category".
ALTER TABLE profile_fields ADD COLUMN accept_mime TEXT NOT NULL DEFAULT '';
/***Statement***/
ALTER TABLE profile_fields ADD COLUMN max_bytes INTEGER NOT NULL DEFAULT 0;
/***Statement***/
-- user_profile_files is the full metadata store for files
-- attached to a workspace-application user profile. Bytes live
-- at data/db/organization/<orgUUID>/uploads/users/<userUUID>/
-- <asset_id>.<ext>. The value stored in
-- user_profiles.profile_data.<field_name> is the asset_id, an
-- opaque key minted here. The media subsystem supplies the
-- magic-byte scanner plus mime and size helpers we reuse, we
-- never write into its media_files table for per-user profile
-- uploads, keeping this data isolated inside the per-org
-- profile database.
CREATE TABLE IF NOT EXISTS user_profile_files
(
    asset_id   TEXT NOT NULL CONSTRAINT user_profile_files_pk PRIMARY KEY UNIQUE,
    user_id    TEXT NOT NULL,
    field_name TEXT NOT NULL,
    filename   TEXT NOT NULL,
    mime_type  TEXT NOT NULL,
    size_bytes INTEGER NOT NULL,
    ext        TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
/***Statement***/
CREATE INDEX IF NOT EXISTS user_profile_files_user_field_idx
    ON user_profile_files (user_id, field_name);
