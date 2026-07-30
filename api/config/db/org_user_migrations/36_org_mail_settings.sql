/***Statement***/
-- Per-organization event to template/account bindings and activation
-- cadence overrides - mirrors the global mail.db mail_settings KV
-- table. An unset key falls back to the global mail_settings row of
-- the same key - see api/src/mail/scopedsettings.
-- See plan/org-app-email-settings/plan.md.
CREATE TABLE IF NOT EXISTS org_mail_settings
(
    key        TEXT NOT NULL CONSTRAINT org_mail_settings_pk PRIMARY KEY,
    value      TEXT NOT NULL DEFAULT '',
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
