CREATE TABLE IF NOT EXISTS drives (
    id         TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(8)))),
    name       TEXT NOT NULL UNIQUE,
    type       TEXT NOT NULL DEFAULT 'S3' CHECK(type IN ('s3')),
    endpoint   TEXT NOT NULL DEFAULT '',
    bucket     TEXT NOT NULL DEFAULT '',
    region     TEXT NOT NULL DEFAULT '',
    access_key TEXT NOT NULL DEFAULT '',
    secret_key TEXT NOT NULL DEFAULT '',
    prefix     TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);
