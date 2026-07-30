/***Statement***/
-- Per-organization SMTP accounts - mirrors the global mail.db mail_smtp_accounts
-- table exactly, scoped by living in the org own users.db instead. An org with
-- no rows here (or no active row) falls back to the global mail engine active
-- account - see api/src/mail/scopedsettings. See plan/org-app-email-settings/plan.md.
CREATE TABLE IF NOT EXISTS org_mail_smtp_accounts
(
    id         TEXT    NOT NULL CONSTRAINT org_mail_smtp_accounts_pk PRIMARY KEY,
    name       TEXT    NOT NULL,
    host       TEXT    NOT NULL,
    port       INTEGER NOT NULL DEFAULT 587,
    username   TEXT    NOT NULL DEFAULT '',
    password   TEXT    NOT NULL DEFAULT '',
    from_name  TEXT    NOT NULL DEFAULT '',
    from_email TEXT    NOT NULL DEFAULT '',
    use_tls    BOOLEAN NOT NULL DEFAULT FALSE,
    is_active  BOOLEAN NOT NULL DEFAULT FALSE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
/***Statement***/
CREATE UNIQUE INDEX IF NOT EXISTS org_mail_smtp_accounts_name_uidx ON org_mail_smtp_accounts (name);
/***Statement***/
CREATE UNIQUE INDEX IF NOT EXISTS org_mail_smtp_accounts_active_uidx ON org_mail_smtp_accounts (is_active) WHERE is_active = TRUE;
