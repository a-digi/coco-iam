/***Statement***/
-- Immutable, human-readable identifier for an application, used to
-- name its per-application login-log database (<slug>_login.db). Only
-- unique within this org DB by itself - global uniqueness comes from
-- the application_slugs reservation table in the main DB. See
-- plan/login-audit-log/plan.md Step 5.
ALTER TABLE applications ADD COLUMN slug TEXT;
/***Statement***/
CREATE UNIQUE INDEX IF NOT EXISTS applications_slug_uindex ON applications (slug);
