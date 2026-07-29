/***Statement***/
-- First-observed request body for this attack target, size-capped and
-- with known-sensitive JSON/form keys redacted before storage. Null
-- for GET/HEAD hits and for targets never carrying a captured body.
-- See plan/attack-ip-attribution/plan.md Fix 2.
ALTER TABLE ip_attack_targets ADD COLUMN body_sample TEXT;
/***Statement***/
-- Snapshot of every candidate client-identifying header present on an
-- episode opening hit, captured only when the resolved ip ended up
-- loopback or private (the fallback header chain found nothing valid)
-- so an operator can still trace the real source by hand. Null in the
-- normal case where ip was resolved successfully.
-- See plan/attack-ip-attribution/plan.md Fix 3.
ALTER TABLE ip_attacks ADD COLUMN origin_hint TEXT;
