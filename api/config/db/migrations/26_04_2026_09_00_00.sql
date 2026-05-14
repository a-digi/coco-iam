/***Statement***/
CREATE TABLE IF NOT EXISTS application_keys (
    id              TEXT PRIMARY KEY,
    application_id  TEXT NOT NULL,
    status          TEXT NOT NULL,
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    activated_at    DATETIME,
    deactivated_at  DATETIME,
    expires_at      DATETIME
);
/***Statement***/
CREATE INDEX IF NOT EXISTS application_keys_app_idx ON application_keys(application_id);
/***Statement***/
CREATE INDEX IF NOT EXISTS application_keys_app_status_idx ON application_keys(application_id, status);
