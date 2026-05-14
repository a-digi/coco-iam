/***Statement***/
-- user_oauth_identities maps (provider, provider_sub) → one of
-- the per-org users.id rows. A single user can have multiple
-- linked identities (Google and GitHub on the same account), so
-- lookup is always by the pair (provider, provider_sub) and never
-- by email alone.
--
-- email_at_link is the email the IdP returned at link time,
-- kept for audit. It may drift from the current users.email row
-- later if the user changes either one.
--
-- email_verified_at_link captures whether the IdP told us the
-- email was verified at link time. Only verified=true emails are
-- eligible for the account-linking by email-match path. Others
-- create new accounts.
CREATE TABLE IF NOT EXISTS user_oauth_identities
(
    id                      TEXT NOT NULL CONSTRAINT user_oauth_identities_pk PRIMARY KEY UNIQUE,
    user_id                 TEXT NOT NULL,
    provider                TEXT NOT NULL,
    provider_sub            TEXT NOT NULL,
    email_at_link           TEXT NOT NULL DEFAULT '',
    email_verified_at_link  INTEGER NOT NULL DEFAULT 0,
    created_at              DATETIME DEFAULT CURRENT_TIMESTAMP
);
/***Statement***/
CREATE UNIQUE INDEX IF NOT EXISTS user_oauth_identities_provider_sub_idx
    ON user_oauth_identities (provider, provider_sub);
/***Statement***/
CREATE INDEX IF NOT EXISTS user_oauth_identities_user_id_idx
    ON user_oauth_identities (user_id);
