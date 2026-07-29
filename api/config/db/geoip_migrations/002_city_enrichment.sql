/***Statement***/
-- City-level enrichment (city name, subdivision/state, postal code,
-- lat/long), sourced from MaxMind's GeoLite2-City-CSV — a strict
-- superset of the Country edition, which is why the updater pulls
-- City instead of Country now. geoip_country_ranges (001_initial.sql)
-- stays in the schema (migrations are append-only) but is no longer
-- populated — harmless, since geoip.db is rebuilt wholesale from an
-- empty file on every pull, not accumulated in place. See
-- plan/geoip-enrichment/plan.md's "Extension: city-level GeoIP"
-- section.
CREATE TABLE IF NOT EXISTS geoip_city_ranges
(
    id           INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    family       TEXT NOT NULL,
    start_ip     TEXT NOT NULL,
    end_ip       TEXT NOT NULL,
    country_code TEXT,
    country_name TEXT,
    subdivision  TEXT,
    city         TEXT,
    postal_code  TEXT,
    latitude     REAL,
    longitude    REAL
);
/***Statement***/
CREATE INDEX IF NOT EXISTS geoip_city_ranges_lookup_idx ON geoip_city_ranges (family, start_ip DESC);
