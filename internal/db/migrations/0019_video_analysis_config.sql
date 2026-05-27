CREATE TABLE IF NOT EXISTS video_analysis_config (
    id                  INTEGER PRIMARY KEY CHECK(id = 1),
    enabled             INTEGER NOT NULL DEFAULT 0,
    service_url         TEXT    NOT NULL DEFAULT '',
    model               TEXT    NOT NULL DEFAULT 'yolov8n',
    confidence_threshold REAL   NOT NULL DEFAULT 0.4
);

INSERT OR IGNORE INTO video_analysis_config (id, enabled, service_url, model, confidence_threshold)
VALUES (1, 0, '', 'yolov8n', 0.4);
