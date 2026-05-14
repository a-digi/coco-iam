/***Statement***/
CREATE TABLE IF NOT EXISTS profile_fields
(
    id TEXT NOT NULL CONSTRAINT profile_fields_pk PRIMARY KEY UNIQUE,
    name TEXT NOT NULL,
    label TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    data_type TEXT NOT NULL,
    is_required BOOLEAN NOT NULL DEFAULT 0,
    min_value INTEGER,
    max_value INTEGER,
    options_json TEXT NOT NULL DEFAULT '[]',
    regex TEXT NOT NULL DEFAULT '',
    order_index INTEGER NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
/***Statement***/
CREATE UNIQUE INDEX IF NOT EXISTS profile_fields_name_unique_idx ON profile_fields (name);
/***Statement***/
CREATE INDEX IF NOT EXISTS profile_fields_order_idx ON profile_fields (order_index);
/***Statement***/
CREATE INDEX IF NOT EXISTS profile_fields_is_active_idx ON profile_fields (is_active);
