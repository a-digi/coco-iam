/***Statement***/
-- Per-application email templates - mirrors org_mail_templates (step 1),
-- scoped by application_id since applications share their org's users.db.
-- Missing/inactive falls back to the org's own template of the same
-- name, then the global one - see api/src/mail/scopedsettings.
-- See plan/org-app-email-settings/plan.md.
CREATE TABLE IF NOT EXISTS app_mail_templates
(
    id             TEXT    NOT NULL CONSTRAINT app_mail_templates_pk PRIMARY KEY,
    application_id TEXT    NOT NULL,
    name           TEXT    NOT NULL,
    description    TEXT    NOT NULL DEFAULT '',
    subject        TEXT    NOT NULL DEFAULT '',
    text_body      TEXT    NOT NULL DEFAULT '',
    html_body      TEXT    NOT NULL DEFAULT '',
    is_active      BOOLEAN NOT NULL DEFAULT TRUE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
/***Statement***/
CREATE UNIQUE INDEX IF NOT EXISTS app_mail_templates_app_name_uidx ON app_mail_templates (application_id, name);
/***Statement***/
CREATE INDEX IF NOT EXISTS app_mail_templates_app_id_index ON app_mail_templates (application_id);
