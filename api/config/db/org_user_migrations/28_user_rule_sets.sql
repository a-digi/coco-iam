/***Statement***/
CREATE TABLE IF NOT EXISTS user_rule_sets (
    id TEXT PRIMARY KEY,
    rules_json TEXT NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
