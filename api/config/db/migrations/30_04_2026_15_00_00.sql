/***Statement***/
-- allow_password_login gates the workspace-application legacy
-- username+password login endpoint. Default 1 preserves the
-- existing behaviour for rows migrated from before this column
-- existed and for future rows where the admin leaves it alone.
-- Admins toggle it to 0 in the Authentication tab to force
-- OAuth-only auth.
--
-- The backend enforces the flag in two places:
--   - POST /api/v1/applications/authenticate rejects the
--     request when the flag is 0 (no password login is
--     possible).
--   - GET /api/v1/public/applications/auth-methods omits the
--     "password" entry so the SPA hides the password input.
ALTER TABLE applications ADD COLUMN allow_password_login BOOLEAN NOT NULL DEFAULT 1;
