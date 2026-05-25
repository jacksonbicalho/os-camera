CREATE TABLE IF NOT EXISTS retention_config (
    category TEXT PRIMARY KEY CHECK(category IN ('with_motion', 'without_motion')),
    action   TEXT NOT NULL DEFAULT 'delete' CHECK(action IN ('delete', 'send_to_drive')),
    drive_id TEXT REFERENCES drives(id) ON DELETE SET DEFAULT
);

INSERT OR IGNORE INTO retention_config (category, action) VALUES ('with_motion', 'delete');
INSERT OR IGNORE INTO retention_config (category, action) VALUES ('without_motion', 'delete');
