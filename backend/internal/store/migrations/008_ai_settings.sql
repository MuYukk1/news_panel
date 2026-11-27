CREATE TABLE ai_settings (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    decision_enabled INTEGER,
    decision_provider TEXT,
    decision_base_url TEXT,
    decision_api_key TEXT,
    decision_account_id TEXT,
    decision_auth_header TEXT,
    decision_model TEXT,
    decision_json_format INTEGER,
    decision_timeout_seconds INTEGER,
    decision_l3_priorities TEXT,
    decision_low_action TEXT,
    decision_fallback_mode TEXT,
    topics TEXT,
    rss_max_items_per_pull INTEGER,
    updated_at INTEGER NOT NULL DEFAULT (unixepoch())
);
