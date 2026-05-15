CREATE TABLE IF NOT EXISTS recordings (
    id         INTEGER  PRIMARY KEY AUTOINCREMENT,
    camera_id  TEXT     NOT NULL REFERENCES cameras(id) ON DELETE CASCADE,
    started_at DATETIME NOT NULL,
    ended_at   DATETIME,
    path       TEXT     NOT NULL UNIQUE,
    size_bytes INTEGER  NOT NULL DEFAULT 0,
    has_motion INTEGER  NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_recordings_camera_started ON recordings(camera_id, started_at);
