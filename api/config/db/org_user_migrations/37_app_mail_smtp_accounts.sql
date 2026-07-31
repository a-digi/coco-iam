/***Statement***/
-- Per-application SMTP accounts - mirrors org_mail_smtp_accounts (step 1),
-- one level deeper: applications live inside the SAME org users.db as
-- other applications, so every row is scoped by application_id (unlike
-- the org tier, which is scoped implicitly by living in its own DB). An
-- app with no rows here (or no active row) falls back to the org's own
-- active account, then the global one - see api/src/mail/scopedsettings.
-- See plan/org-app-email-settings/plan.md.
CREATE TABLE IF NOT EXISTS app_mail_smtp_accounts
(
    id             TEXT    NOT NULL CONSTRAINT app_mail_smtp_accounts_pk PRIMARY KEY,
    application_id TEXT    NOT NULL,
    name           TEXT    NOT NULL,
    host           TEXT    NOT NULL,
    port           INTEGER NOT NULL DEFAULT 587,
    username       TEXT    NOT NULL DEFAULT '',
    password       TEXT    NOT NULL DEFAULT '',
    from_name      TEXT    NOT NULL DEFAULT '',
    from_email     TEXT    NOT NULL DEFAULT '',
    use_tls        BOOLEAN NOT NULL DEFAULT FALSE,
    is_active      BOOLEAN NOT NULL DEFAULT FALSE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
/***Statement***/
CREATE UNIQUE INDEX IF NOT EXISTS app_mail_smtp_accounts_app_name_uidx ON app_mail_smtp_accounts (application_id, name);
/***Statement***/
CREATE UNIQUE INDEX IF NOT EXISTS app_mail_smtp_accounts_app_active_uidx ON app_mail_smtp_accounts (application_id) WHERE is_active = TRUE;
/***Statement***/
CREATE INDEX IF NOT EXISTS app_mail_smtp_accounts_app_id_index ON app_mail_smtp_accounts (application_id);
