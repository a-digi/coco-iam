/***Statement***/
-- Two tables, co-located with profile_fields in the per-org
-- profiles.db. A single-step registration is just an application
-- with one step row, multi-step is N > 1. Every registration field
-- belongs to exactly one step via step_id. SQLite does not enforce
-- the step_id FK at the engine level -- the repository enforces it
-- in application code on every atomic-replace.
CREATE TABLE IF NOT EXISTS application_registration_steps
(
    id             TEXT NOT NULL CONSTRAINT application_registration_steps_pk PRIMARY KEY UNIQUE,
    application_id TEXT NOT NULL,
    order_index    INTEGER NOT NULL DEFAULT 0,
    title          TEXT NOT NULL DEFAULT '',
    description    TEXT NOT NULL DEFAULT '',
    created_at     DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at     DATETIME DEFAULT CURRENT_TIMESTAMP
);
/***Statement***/
CREATE INDEX IF NOT EXISTS application_registration_steps_app_idx
    ON application_registration_steps (application_id);
/***Statement***/
CREATE INDEX IF NOT EXISTS application_registration_steps_order_idx
    ON application_registration_steps (application_id, order_index);
/***Statement***/
CREATE TABLE IF NOT EXISTS application_registration_fields
(
    id                TEXT NOT NULL CONSTRAINT application_registration_fields_pk PRIMARY KEY UNIQUE,
    application_id    TEXT NOT NULL,
    step_id           TEXT NOT NULL,
    order_index       INTEGER NOT NULL DEFAULT 0,
    source            TEXT NOT NULL CHECK (source IN ('profile','custom')),
    profile_field_id  TEXT,
    required_override BOOLEAN,
    name              TEXT,
    label             TEXT,
    description       TEXT NOT NULL DEFAULT '',
    data_type         TEXT,
    is_required       BOOLEAN NOT NULL DEFAULT 0,
    min_value         INTEGER,
    max_value         INTEGER,
    options_json      TEXT NOT NULL DEFAULT '[]',
    regex             TEXT NOT NULL DEFAULT '',
    created_at        DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at        DATETIME DEFAULT CURRENT_TIMESTAMP
);
/***Statement***/
CREATE INDEX IF NOT EXISTS application_registration_fields_app_idx
    ON application_registration_fields (application_id);
/***Statement***/
CREATE INDEX IF NOT EXISTS application_registration_fields_step_idx
    ON application_registration_fields (step_id, order_index);
