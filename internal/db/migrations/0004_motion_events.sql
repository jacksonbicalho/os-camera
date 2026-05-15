CREATE TABLE IF NOT EXISTS motion_events (
    id          INTEGER  PRIMARY KEY AUTOINCREMENT,
    camera_id   TEXT     NOT NULL REFERENCES cameras(id) ON DELETE CASCADE,
    occurred_at DATETIME NOT NULL,
    score       REAL     NOT NULL DEFAULT 0,
    frame_path  TEXT,
    bbox_x      REAL,
    bbox_y      REAL,
    bbox_w      REAL,
    bbox_h      REAL
);

CREATE INDEX IF NOT EXISTS idx_motion_events_camera_occurred ON motion_events(camera_id, occurred_at);
