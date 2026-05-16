CREATE TABLE IF NOT EXISTS camera_motion_zones (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    camera_id        TEXT    NOT NULL,
    display_order    INTEGER NOT NULL DEFAULT 0,
    x                REAL    NOT NULL,
    y                REAL    NOT NULL,
    w                REAL    NOT NULL,
    h                REAL    NOT NULL,
    type             TEXT    NOT NULL DEFAULT 'exclude',
    label            TEXT,
    threshold        REAL    NOT NULL DEFAULT 0,
    cooldown_seconds INTEGER NOT NULL DEFAULT 0,
    fps              INTEGER NOT NULL DEFAULT 0,
    scale            REAL    NOT NULL DEFAULT 1,
    FOREIGN KEY (camera_id) REFERENCES cameras(id) ON DELETE CASCADE
);
