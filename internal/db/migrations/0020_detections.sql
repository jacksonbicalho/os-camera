CREATE TABLE IF NOT EXISTS detections (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    recording_id INTEGER NOT NULL REFERENCES recordings(id) ON DELETE CASCADE,
    label        TEXT    NOT NULL,
    confidence   REAL    NOT NULL,
    frame_count  INTEGER NOT NULL DEFAULT 0,
    created_at   DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);

CREATE TABLE IF NOT EXISTS camera_analysis_config (
    camera_id TEXT PRIMARY KEY REFERENCES cameras(id) ON DELETE CASCADE,
    enabled   INTEGER NOT NULL DEFAULT 1
);
