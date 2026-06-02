/***Statement***/
CREATE TABLE application_registration_fields_v2
(
    id                TEXT    NOT NULL PRIMARY KEY UNIQUE,
    application_id    TEXT    NOT NULL,
    step_id           TEXT    NOT NULL,
    order_index       INTEGER NOT NULL DEFAULT 0,
    source            TEXT    NOT NULL CHECK (source IN ('profile','custom','system')),
    profile_field_id  TEXT,
    required_override BOOLEAN,
    name              TEXT,
    label             TEXT,
    description       TEXT    NOT NULL DEFAULT '',
    data_type         TEXT,
    is_required       BOOLEAN NOT NULL DEFAULT 0,
    min_value         INTEGER,
    max_value         INTEGER,
    options_json      TEXT    NOT NULL DEFAULT '[]',
    regex             TEXT    NOT NULL DEFAULT '',
    system_field_name TEXT CHECK (system_field_name IN ('email','username') OR system_field_name IS NULL),
    created_at        DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at        DATETIME DEFAULT CURRENT_TIMESTAMP
);
/***Statement***/
INSERT INTO application_registration_fields_v2
    SELECT id, application_id, step_id, order_index, source,
           profile_field_id, required_override, name, label,
           description, data_type, is_required, min_value, max_value,
           options_json, regex, NULL, created_at, updated_at
    FROM application_registration_fields;
/***Statement***/
DROP TABLE application_registration_fields;
/***Statement***/
ALTER TABLE application_registration_fields_v2 RENAME TO application_registration_fields;
/***Statement***/
CREATE INDEX IF NOT EXISTS application_registration_fields_app_idx
    ON application_registration_fields (application_id);
/***Statement***/
CREATE INDEX IF NOT EXISTS application_registration_fields_step_idx
    ON application_registration_fields (step_id, order_index);
