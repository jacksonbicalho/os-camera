-- Device info is stored as a key/value snapshot per camera (EAV) so different
-- camera models can keep whatever characteristics they expose (identity, NTP,
-- per-stream encode, capabilities, config URLs, raw dump) without schema churn.
CREATE TABLE IF NOT EXISTS camera_device_info (
    camera_id    TEXT     NOT NULL REFERENCES cameras(id) ON DELETE CASCADE,
    key          TEXT     NOT NULL,
    value        TEXT     NOT NULL DEFAULT '',
    collected_at DATETIME NOT NULL,
    PRIMARY KEY (camera_id, key)
);
