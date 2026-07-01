-- Optional per-camera RTSP URL for motion detection (e.g. a lighter substream).
-- Empty string means "use the main rtsp_url" (no behavior change).
ALTER TABLE cameras ADD COLUMN motion_rtsp_url TEXT NOT NULL DEFAULT '';
