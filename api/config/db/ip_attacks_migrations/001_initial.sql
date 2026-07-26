/***Statement***/
-- One row per distinct attack episode from an IP — created when a
-- ban first fires for that IP with no other episode currently open,
-- updated (hit_count, last_seen_at, ban_count) as the episode
-- continues, closed (ended_at set) once activity has been quiet past
-- the grace period. A later attack from the same IP is a new row, not
-- a reopened one. See plan/ip-abuse-protection/plan.md section 10.
CREATE TABLE IF NOT EXISTS ip_attacks
(
    id           TEXT NOT NULL CONSTRAINT ip_attacks_pk PRIMARY KEY,
    ip           TEXT NOT NULL,
    tier         TEXT NOT NULL,
    started_at   DATETIME NOT NULL,
    last_seen_at DATETIME NOT NULL,
    ended_at     DATETIME,
    hit_count    INTEGER NOT NULL DEFAULT 0,
    ban_count    INTEGER NOT NULL DEFAULT 1
);
/***Statement***/
CREATE INDEX IF NOT EXISTS ip_attacks_ip_idx ON ip_attacks (ip);
/***Statement***/
CREATE INDEX IF NOT EXISTS ip_attacks_open_idx ON ip_attacks (ended_at);
/***Statement***/
CREATE INDEX IF NOT EXISTS ip_attacks_started_at_idx ON ip_attacks (started_at DESC);
/***Statement***/
-- Aggregated per-endpoint hit counts within one attack episode —
-- what the admin Attacks page drills into for a single row.
CREATE TABLE IF NOT EXISTS ip_attack_targets
(
    id        TEXT NOT NULL CONSTRAINT ip_attack_targets_pk PRIMARY KEY,
    attack_id TEXT NOT NULL CONSTRAINT ip_attack_targets_attack_fk REFERENCES ip_attacks (id),
    path      TEXT NOT NULL,
    method    TEXT NOT NULL,
    hit_count INTEGER NOT NULL DEFAULT 0
);
/***Statement***/
CREATE UNIQUE INDEX IF NOT EXISTS ip_attack_targets_unique_idx ON ip_attack_targets (attack_id, path, method);
/***Statement***/
CREATE INDEX IF NOT EXISTS ip_attack_targets_attack_id_idx ON ip_attack_targets (attack_id);
