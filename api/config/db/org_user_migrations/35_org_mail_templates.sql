/***Statement***/
-- Per-organization email templates - mirrors the global mail.db mail_templates
-- table. Missing/inactive falls back to the global template of the same event
-- name - see api/src/mail/scopedsettings. See plan/org-app-email-settings/plan.md.
CREATE TABLE IF NOT EXISTS org_mail_templates
(
    id          TEXT    NOT NULL CONSTRAINT org_mail_templates_pk PRIMARY KEY,
    name        TEXT    NOT NULL,
    description TEXT    NOT NULL DEFAULT '',
    subject     TEXT    NOT NULL DEFAULT '',
    text_body   TEXT    NOT NULL DEFAULT '',
    html_body   TEXT    NOT NULL DEFAULT '',
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
/***Statement***/
CREATE UNIQUE INDEX IF NOT EXISTS org_mail_templates_name_uidx ON org_mail_templates (name);
