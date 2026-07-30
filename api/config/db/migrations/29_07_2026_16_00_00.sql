/***Statement***/
-- Admin-editable failed-login ban-rule settings - lives in the main
-- DB, singleton row (id always 1, no multi-tenancy needed), same
-- pattern as geoip_settings. Both *_enabled default to 0 (disabled) -
-- deliberately opt-in, since an unreviewed default threshold could
-- auto-ban real traffic (or an admin's own IP) the moment this
-- feature deploys. See plan/login-ban-rules/plan.md.
CREATE TABLE IF NOT EXISTS login_ban_rules
(
    id                         INTEGER NOT NULL PRIMARY KEY CHECK (id = 1),
    admin_enabled              INTEGER NOT NULL DEFAULT 0,
    admin_threshold            INTEGER NOT NULL DEFAULT 5,
    admin_window_seconds       INTEGER NOT NULL DEFAULT 600,
    admin_ban_seconds          INTEGER NOT NULL DEFAULT 3600,
    application_enabled        INTEGER NOT NULL DEFAULT 0,
    application_threshold      INTEGER NOT NULL DEFAULT 5,
    application_window_seconds INTEGER NOT NULL DEFAULT 600,
    application_ban_seconds    INTEGER NOT NULL DEFAULT 3600,
    updated_at                 DATETIME
);
