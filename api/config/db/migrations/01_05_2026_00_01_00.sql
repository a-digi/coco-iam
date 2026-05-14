/***Statement***/
ALTER TABLE user_activations ADD COLUMN org_id TEXT NOT NULL DEFAULT '';
/***Statement***/
UPDATE user_activations SET org_id = COALESCE(
    (SELECT organization_id FROM users WHERE users.id = user_activations.user_id LIMIT 1), ''
);
