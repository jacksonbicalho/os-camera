CREATE TABLE IF NOT EXISTS users (
    id            INTEGER  PRIMARY KEY AUTOINCREMENT,
    username      TEXT     UNIQUE NOT NULL,
    password_hash TEXT     NOT NULL,
    role          TEXT     NOT NULL CHECK(role IN ('admin','viewer')),
    created_at    DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS user_cameras (
    user_id   INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    camera_id TEXT    NOT NULL,
    PRIMARY KEY (user_id, camera_id)
);

CREATE TABLE IF NOT EXISTS cameras (
    id                 TEXT PRIMARY KEY,
    rtsp_url           TEXT NOT NULL,
    chunk_duration     TEXT NOT NULL DEFAULT '5m',
    reconnect_interval TEXT NOT NULL DEFAULT '30s',
    video_codec        TEXT,
    has_audio          INTEGER,
    width              INTEGER,
    height             INTEGER,
    display_order      INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS camera_motion (
    camera_id        TEXT    PRIMARY KEY REFERENCES cameras(id) ON DELETE CASCADE,
    enabled          INTEGER NOT NULL DEFAULT 0,
    threshold        REAL    NOT NULL DEFAULT 0.02,
    fps              INTEGER NOT NULL DEFAULT 2,
    cooldown_seconds INTEGER NOT NULL DEFAULT 30
);

CREATE TABLE IF NOT EXISTS system_config (
    key   TEXT PRIMARY KEY,
    value TEXT
);
