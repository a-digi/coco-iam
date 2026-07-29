/***Statement***/
-- One row per IPv4 or IPv6 country allocation block. Rebuilt wholesale
-- on every successful pull by the geoip-updater executable - the
-- whole table is populated fresh into a brand-new file and atomically
-- swapped into place, never updated in place. start_ip/end_ip are
-- fixed-width zero-padded hex text (8 hex chars for v4, 32 for v6) so
-- lexicographic ordering equals numeric ordering. See
-- plan/geoip-enrichment/plan.md.
CREATE TABLE IF NOT EXISTS geoip_country_ranges
(
    id           INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    family       TEXT NOT NULL,
    start_ip     TEXT NOT NULL,
    end_ip       TEXT NOT NULL,
    country_code TEXT,
    country_name TEXT
);
/***Statement***/
CREATE INDEX IF NOT EXISTS geoip_country_ranges_lookup_idx ON geoip_country_ranges (family, start_ip DESC);
/***Statement***/
-- One row per IPv4 or IPv6 ASN allocation block. Same fresh-rebuild
-- and encoding discipline as geoip_country_ranges above.
CREATE TABLE IF NOT EXISTS geoip_asn_ranges
(
    id       INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    family   TEXT NOT NULL,
    start_ip TEXT NOT NULL,
    end_ip   TEXT NOT NULL,
    asn      INTEGER,
    as_org   TEXT
);
/***Statement***/
CREATE INDEX IF NOT EXISTS geoip_asn_ranges_lookup_idx ON geoip_asn_ranges (family, start_ip DESC);
/***Statement***/
-- Small key/value table for this generation own bookkeeping, e.g.
-- last_pulled_at - written by the updater into the same fresh file it
-- just built, before the atomic rename.
CREATE TABLE IF NOT EXISTS geoip_meta
(
    key   TEXT NOT NULL PRIMARY KEY,
    value TEXT NOT NULL
);
