/***Statement***/
-- Admin-editable GeoIP settings - lives in the main DB, deliberately
-- not geoip.db, since geoip.db is rebuilt wholesale and atomically
-- swapped on every successful pull (no history kept by design). A
-- setting stored there would be silently wiped the next time the
-- updater runs. Singleton row (id always 1, no multi-tenancy needed).
-- See plan/geoip-enrichment/plan.md.
CREATE TABLE IF NOT EXISTS geoip_settings
(
    id                     INTEGER NOT NULL PRIMARY KEY CHECK (id = 1),
    enabled                INTEGER NOT NULL DEFAULT 0,
    maxmind_account_id     TEXT NOT NULL DEFAULT '',
    maxmind_license_key    TEXT NOT NULL DEFAULT '',
    check_interval_seconds INTEGER NOT NULL DEFAULT 600,
    pull_interval_hours    INTEGER NOT NULL DEFAULT 24,
    updated_at             DATETIME
);
