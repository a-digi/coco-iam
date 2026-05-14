/***Statement***/
ALTER TABLE applications ADD COLUMN allow_recovery     BOOLEAN NOT NULL DEFAULT 1;
/***Statement***/
ALTER TABLE applications ADD COLUMN allow_registration BOOLEAN NOT NULL DEFAULT 0;
/***Statement***/
ALTER TABLE application_login_settings DROP COLUMN allow_recovery;
/***Statement***/
ALTER TABLE application_login_settings DROP COLUMN allow_registration;
