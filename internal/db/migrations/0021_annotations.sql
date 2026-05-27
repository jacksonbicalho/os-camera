CREATE TABLE IF NOT EXISTS annotations (
    id         INTEGER  PRIMARY KEY AUTOINCREMENT,
    event_id   INTEGER  NOT NULL REFERENCES motion_events(id) ON DELETE CASCADE,
    label      TEXT     NOT NULL,
    bbox_x     REAL     NOT NULL,
    bbox_y     REAL     NOT NULL,
    bbox_w     REAL     NOT NULL,
    bbox_h     REAL     NOT NULL,
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

CREATE INDEX IF NOT EXISTS idx_annotations_event ON annotations(event_id);
