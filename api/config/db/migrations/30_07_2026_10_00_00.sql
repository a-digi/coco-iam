/***Statement***/
-- Admin-editable ban rule for high-volume scan/probe traffic (many
-- requests to nonexistent routes - the "unmatched" attack tier, see
-- ipguard.go's RecordRecon/recordAttackHit). Unlike login_ban_rules
-- this is a single global rule, not split by domain - scan traffic
-- is not scoped to admin vs application logins. Disabled by default,
-- deliberately opt-in. See plan/attack-ban-rules/plan.md.
CREATE TABLE IF NOT EXISTS attack_ban_rules
(
    id             INTEGER NOT NULL PRIMARY KEY CHECK (id = 1),
    enabled        INTEGER NOT NULL DEFAULT 0,
    threshold      INTEGER NOT NULL DEFAULT 50,
    window_seconds INTEGER NOT NULL DEFAULT 60,
    ban_seconds    INTEGER NOT NULL DEFAULT 3600,
    updated_at     DATETIME
);
