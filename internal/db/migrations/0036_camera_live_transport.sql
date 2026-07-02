-- Per-camera live transport preference: auto (try WebRTC, fall back to HLS),
-- webrtc (prefer WebRTC), or hls (force HLS, no WebRTC attempt).
-- Default auto preserves the current behavior.
ALTER TABLE cameras ADD COLUMN live_transport TEXT NOT NULL DEFAULT 'auto';
