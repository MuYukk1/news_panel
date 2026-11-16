-- Preserve the upstream RSS index (feed_id, guid); empty plugin keys are excluded.
ALTER TABLE items ADD COLUMN source_id TEXT NOT NULL DEFAULT '';
ALTER TABLE items ADD COLUMN external_id TEXT NOT NULL DEFAULT '';
ALTER TABLE items ADD COLUMN source_type TEXT NOT NULL DEFAULT '';
ALTER TABLE items ADD COLUMN published_at TEXT NOT NULL DEFAULT '';
ALTER TABLE items ADD COLUMN author TEXT NOT NULL DEFAULT '';
ALTER TABLE items ADD COLUMN raw_meta TEXT NOT NULL DEFAULT '{}';
ALTER TABLE items ADD COLUMN domain TEXT NOT NULL DEFAULT 'other';
ALTER TABLE items ADD COLUMN ai_score INTEGER CHECK (ai_score BETWEEN 0 AND 100);
ALTER TABLE items ADD COLUMN ai_tags TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(ai_tags) AND json_type(ai_tags) = 'array');
ALTER TABLE items ADD COLUMN ai_summary TEXT NOT NULL DEFAULT '';
ALTER TABLE items ADD COLUMN priority TEXT NOT NULL DEFAULT 'normal' CHECK (priority IN ('critical','high','normal','low'));
ALTER TABLE items ADD COLUMN triage_status TEXT NOT NULL DEFAULT 'pending' CHECK (triage_status IN ('pending','kept','filtered','failed'));
CREATE UNIQUE INDEX idx_items_source_external ON items(source_id, external_id)
    WHERE source_id != '' AND external_id != '';
CREATE INDEX idx_items_triage ON items(triage_status, pub_date DESC);

CREATE TABLE plugin_heartbeats (
    source_id TEXT PRIMARY KEY,
    plugin_version TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('ok','degraded','error')),
    fetched_count INTEGER NOT NULL CHECK (fetched_count >= 0),
    error_message TEXT,
    timestamp TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

-- RSS predates the triage pipeline and remains visible in the default kept view.
UPDATE items SET triage_status = 'kept' WHERE source_id = '';
