/***Statement***/
CREATE TABLE IF NOT EXISTS application_recovery_templates (
    application_id TEXT PRIMARY KEY,
    request_body_html TEXT NOT NULL DEFAULT '',
    reset_body_html TEXT NOT NULL DEFAULT '',
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
