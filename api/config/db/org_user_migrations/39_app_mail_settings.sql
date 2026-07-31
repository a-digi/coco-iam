/***Statement***/
-- Per-application event to template/account bindings and activation
-- cadence overrides - mirrors org_mail_settings (step 1), scoped by
-- application_id since applications share their org's users.db. An
-- unset key falls back to the org's own mail_settings row of the same
-- key, then the global one - see api/src/mail/scopedsettings.
-- See plan/org-app-email-settings/plan.md.
CREATE TABLE IF NOT EXISTS app_mail_settings
(
    application_id TEXT NOT NULL,
    key            TEXT NOT NULL,
    value          TEXT NOT NULL DEFAULT '',
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT app_mail_settings_pk PRIMARY KEY (application_id, key)
);
