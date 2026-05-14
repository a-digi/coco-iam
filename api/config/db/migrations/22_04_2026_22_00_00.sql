/***Statement***/
-- Per-org user-database rollout flag. Starts FALSE for every existing
-- organization so the one-shot data migration picks them up. Flipped
-- to TRUE once the org's slice of users / user_auth_password /
-- user_acl / application_user_acl / user_group_members has been
-- copied into ./data/db/org_users_<id>.db. A later migration drops
-- the legacy main-DB tables once every org is flagged TRUE.
ALTER TABLE organization ADD COLUMN users_migrated_to_org_db BOOLEAN NOT NULL DEFAULT FALSE;
