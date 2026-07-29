/***Statement***/
-- Global reservation of application slugs. Applications live in each
-- organization's own users.db, so a plain SQL UNIQUE index there (see
-- org_user_migrations/32_application_slug.sql) can only guarantee
-- per-org uniqueness. This table is the actual cross-org uniqueness
-- guarantee: every application-creation flow reserves its candidate
-- slug here, transactionally, before writing the row into the org DB.
-- Never exposed via any public API - its sole purpose is guaranteeing
-- a collision-free slug for a per-application login-log database's
-- file path. See plan/login-audit-log/plan.md Step 5.
CREATE TABLE IF NOT EXISTS application_slugs
(
    slug            TEXT NOT NULL CONSTRAINT application_slugs_pk PRIMARY KEY,
    application_id  TEXT NOT NULL,
    organization_id TEXT NOT NULL,
    created_at      DATETIME NOT NULL
);
