/***Statement***/
-- Snapshot of what the geoip package resolved for this episode ip at
-- the moment it opened (a JSON-encoded geoip.Info), captured once and
-- never updated afterward. Never re-derived from a live geoip.db
-- lookup at read time - geoip.db keeps no history of its own (every
-- refresh replaces the whole file), so this column is the only place
-- that fact survives. Null if geoip was disabled, the ip was
-- loopback or private, or nothing matched. See
-- plan/geoip-enrichment/plan.md.
ALTER TABLE ip_attacks ADD COLUMN geoip_info TEXT;
/***Statement***/
-- Same snapshot, same reasoning, for port-scan episodes - see
-- scan_episodes in 003_scan_episodes.sql.
ALTER TABLE scan_episodes ADD COLUMN geoip_info TEXT;
