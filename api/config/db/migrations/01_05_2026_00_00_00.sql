/***Statement***/
CREATE TABLE IF NOT EXISTS user_org_index (
    user_id TEXT NOT NULL CONSTRAINT user_org_index_pk PRIMARY KEY UNIQUE,
    org_id  TEXT NOT NULL
);
/***Statement***/
CREATE INDEX IF NOT EXISTS user_org_index_org_idx ON user_org_index (org_id);
/***Statement***/
CREATE TABLE IF NOT EXISTS user_email_org_index (
    email   TEXT NOT NULL,
    org_id  TEXT NOT NULL,
    user_id TEXT NOT NULL,
    PRIMARY KEY (email, org_id)
);
/***Statement***/
CREATE INDEX IF NOT EXISTS user_email_org_idx ON user_email_org_index (email);
/***Statement***/
INSERT OR IGNORE INTO user_org_index (user_id, org_id)
SELECT id, organization_id FROM users WHERE organization_id IS NOT NULL AND organization_id != '';
/***Statement***/
INSERT OR IGNORE INTO user_email_org_index (email, org_id, user_id)
SELECT email, organization_id, id FROM users WHERE organization_id IS NOT NULL AND organization_id != '';
