/***Statement***/
-- One row per distinct port-scan episode from an IP — opened once an
-- IP crosses the distinct-port threshold within the aggregation
-- window (a single stray packet to one closed port is noise, not a
-- scan signature), updated as the episode continues, closed after a
-- quiet grace period. Raw per-hit lines are not stored here — they go
-- to port-scans.log, kept out of this small, aggregated-only table
-- the same way ip_attacks keeps out of ip-attacks per-request detail.
-- See plan/port-scan-detection/plan.md Phase B.
CREATE TABLE IF NOT EXISTS scan_episodes
(
    id             TEXT NOT NULL CONSTRAINT scan_episodes_pk PRIMARY KEY,
    ip             TEXT NOT NULL,
    started_at     DATETIME NOT NULL,
    last_seen_at   DATETIME NOT NULL,
    ended_at       DATETIME,
    distinct_ports INTEGER NOT NULL DEFAULT 0,
    hit_count      INTEGER NOT NULL DEFAULT 0,
    sample_ports   TEXT NOT NULL DEFAULT ''
);
/***Statement***/
CREATE INDEX IF NOT EXISTS scan_episodes_ip_idx ON scan_episodes (ip);
/***Statement***/
CREATE INDEX IF NOT EXISTS scan_episodes_open_idx ON scan_episodes (ended_at);
/***Statement***/
CREATE INDEX IF NOT EXISTS scan_episodes_started_at_idx ON scan_episodes (started_at DESC);
