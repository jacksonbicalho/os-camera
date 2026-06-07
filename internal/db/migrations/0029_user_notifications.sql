CREATE TABLE IF NOT EXISTS user_notifications (
    id         INTEGER  PRIMARY KEY AUTOINCREMENT,
    user_id    INTEGER  NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type       TEXT     NOT NULL DEFAULT 'info',
    title      TEXT,
    message    TEXT     NOT NULL,
    link       TEXT,
    created_at DATETIME NOT NULL,
    read_at    DATETIME
);

CREATE INDEX IF NOT EXISTS idx_user_notifications_user_created
    ON user_notifications(user_id, created_at);
