package db

import (
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"camera/internal/config"
)

func generateUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

// CreateCamera inserts a camera row and, if motion is non-nil, its
// camera_motion row. The camera ID is a generated UUID v4.
func CreateCamera(db *DB, cam config.CameraConfig, motion *config.MotionConfig) (config.CameraConfig, error) {
	if cam.ID == "" {
		id, err := generateUUID()
		if err != nil {
			return config.CameraConfig{}, fmt.Errorf("generate camera id: %w", err)
		}
		cam.ID = id
	}

	tx, err := db.Begin()
	if err != nil {
		return config.CameraConfig{}, err
	}
	defer tx.Rollback() //nolint:errcheck

	_, err = tx.Exec(
		`INSERT INTO cameras(id, name, rtsp_url, chunk_duration, reconnect_interval,
		                     video_codec, has_audio, width, height, display_order,
		                     hls_video_mode, record_video_mode, hls_segment_seconds, hls_list_size,
		                     hls_dvr_seconds, recording_enabled)
		 VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		cam.ID,
		cam.Name,
		cam.RTSPURL,
		durationToStr(cam.ChunkDuration, config.DefaultChunkDuration),
		durationToStr(cam.ReconnectInterval, config.DefaultReconnectInterval),
		nullStr(cam.VideoCodec),
		boolPtr(cam.HasAudio),
		nullInt(cam.Width),
		nullInt(cam.Height),
		cam.DisplayOrder,
		cam.HLSVideoMode,
		cam.RecordVideoMode,
		nullIntPtr(cam.HLSSegmentSeconds),
		nullIntPtr(cam.HLSListSize),
		nullIntPtr(cam.HLSDVRSeconds),
		boolToInt(cam.RecordingEnabled),
	)
	if err != nil {
		return config.CameraConfig{}, fmt.Errorf("insert camera: %w", err)
	}

	if motion != nil {
		if err := insertMotion(tx, cam.ID, motion); err != nil {
			return config.CameraConfig{}, err
		}
	}

	if err := tx.Commit(); err != nil {
		return config.CameraConfig{}, err
	}
	return cam, nil
}

// GetCamera returns the camera with the given ID, including its motion config
// if present.
func GetCamera(db *DB, id string) (config.CameraConfig, error) {
	var cam config.CameraConfig
	var chunk, reconnect string
	var codec sql.NullString
	var hasAudio sql.NullInt64
	var width, height sql.NullInt64

	var segSec, listSize, dvrSec sql.NullInt64
	var recEnabled int
	err := db.QueryRow(
		`SELECT id, name, rtsp_url, chunk_duration, reconnect_interval,
		        video_codec, has_audio, width, height, display_order,
		        hls_video_mode, record_video_mode, hls_segment_seconds, hls_list_size,
		        hls_dvr_seconds, recording_enabled
		 FROM cameras WHERE id=?`, id,
	).Scan(
		&cam.ID, &cam.Name, &cam.RTSPURL, &chunk, &reconnect,
		&codec, &hasAudio, &width, &height, &cam.DisplayOrder,
		&cam.HLSVideoMode, &cam.RecordVideoMode, &segSec, &listSize,
		&dvrSec, &recEnabled,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return config.CameraConfig{}, fmt.Errorf("camera %q not found", id)
		}
		return config.CameraConfig{}, fmt.Errorf("get camera: %w", err)
	}

	cam.ChunkDuration = parseDuration(chunk)
	cam.ReconnectInterval = parseDuration(reconnect)
	if codec.Valid {
		cam.VideoCodec = codec.String
	}
	if hasAudio.Valid {
		b := hasAudio.Int64 != 0
		cam.HasAudio = &b
	}
	if width.Valid {
		cam.Width = int(width.Int64)
	}
	if height.Valid {
		cam.Height = int(height.Int64)
	}
	cam.HLSSegmentSeconds = scanIntPtr(segSec)
	cam.HLSListSize = scanIntPtr(listSize)
	cam.HLSDVRSeconds = scanIntPtr(dvrSec)
	cam.RecordingEnabled = recEnabled != 0

	motion, err := getMotion(db.DB, id)
	if err != nil {
		return config.CameraConfig{}, err
	}
	cam.Motion = motion

	return cam, nil
}

// ListCameras returns all cameras ordered by display_order, id.
// Uses a LEFT JOIN to avoid nested queries (single-connection SQLite pool).
func ListCameras(db *DB) ([]config.CameraConfig, error) {
	rows, err := db.Query(`
		SELECT c.id, c.name, c.rtsp_url, c.chunk_duration, c.reconnect_interval,
		       c.video_codec, c.has_audio, c.width, c.height, c.display_order,
		       c.hls_video_mode, c.record_video_mode, c.hls_segment_seconds, c.hls_list_size,
		       c.hls_dvr_seconds, c.recording_enabled,
		       cm.enabled, cm.threshold, cm.fps, cm.cooldown_seconds,
		       cm.capture_width, cm.capture_height, cm.playback_lead_seconds, cm.playback_trail_seconds
		FROM cameras c
		LEFT JOIN camera_motion cm ON cm.camera_id = c.id
		ORDER BY c.display_order, c.id
	`)
	if err != nil {
		return nil, fmt.Errorf("list cameras: %w", err)
	}
	defer rows.Close()

	var cams []config.CameraConfig
	for rows.Next() {
		var cam config.CameraConfig
		var chunk, reconnect string
		var codec sql.NullString
		var hasAudio, width, height, segSec, listSize, dvrSec sql.NullInt64
		var recEnabled int
		var mEnabled sql.NullInt64
		var mThreshold sql.NullFloat64
		var mFPS, mCooldown, mCaptureW, mCaptureH, mPlaybackLead, mPlaybackTrail sql.NullInt64

		if err := rows.Scan(
			&cam.ID, &cam.Name, &cam.RTSPURL, &chunk, &reconnect,
			&codec, &hasAudio, &width, &height, &cam.DisplayOrder,
			&cam.HLSVideoMode, &cam.RecordVideoMode, &segSec, &listSize,
			&dvrSec, &recEnabled,
			&mEnabled, &mThreshold, &mFPS, &mCooldown, &mCaptureW, &mCaptureH, &mPlaybackLead, &mPlaybackTrail,
		); err != nil {
			return nil, fmt.Errorf("scan camera: %w", err)
		}

		cam.ChunkDuration = parseDuration(chunk)
		cam.ReconnectInterval = parseDuration(reconnect)
		if codec.Valid {
			cam.VideoCodec = codec.String
		}
		if hasAudio.Valid {
			b := hasAudio.Int64 != 0
			cam.HasAudio = &b
		}
		if width.Valid {
			cam.Width = int(width.Int64)
		}
		if height.Valid {
			cam.Height = int(height.Int64)
		}
		cam.HLSSegmentSeconds = scanIntPtr(segSec)
		cam.HLSListSize = scanIntPtr(listSize)
		cam.HLSDVRSeconds = scanIntPtr(dvrSec)
		cam.RecordingEnabled = recEnabled != 0
		if mEnabled.Valid {
			cam.Motion = &config.MotionConfig{
				Enabled:             mEnabled.Int64 != 0,
				Threshold:           mThreshold.Float64,
				FPS:                 int(mFPS.Int64),
				CooldownSeconds:     int(mCooldown.Int64),
				CaptureWidth:        int(mCaptureW.Int64),
				CaptureHeight:       int(mCaptureH.Int64),
				PlaybackLeadSeconds:  int(mPlaybackLead.Int64),
				PlaybackTrailSeconds: int(mPlaybackTrail.Int64),
			}
		}

		cams = append(cams, cam)
	}
	return cams, rows.Err()
}

// UpdateCamera updates the camera row and replaces its motion config.
func UpdateCamera(db *DB, cam config.CameraConfig, motion *config.MotionConfig) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	_, err = tx.Exec(
		`UPDATE cameras SET rtsp_url=?, chunk_duration=?, reconnect_interval=?,
		                    video_codec=?, has_audio=?, width=?, height=?, display_order=?,
		                    hls_video_mode=?, record_video_mode=?,
		                    hls_segment_seconds=?, hls_list_size=?, hls_dvr_seconds=?,
		                    recording_enabled=?
		 WHERE id=?`,
		cam.RTSPURL,
		durationToStr(cam.ChunkDuration, config.DefaultChunkDuration),
		durationToStr(cam.ReconnectInterval, config.DefaultReconnectInterval),
		nullStr(cam.VideoCodec),
		boolPtr(cam.HasAudio),
		nullInt(cam.Width),
		nullInt(cam.Height),
		cam.DisplayOrder,
		cam.HLSVideoMode,
		cam.RecordVideoMode,
		nullIntPtr(cam.HLSSegmentSeconds),
		nullIntPtr(cam.HLSListSize),
		nullIntPtr(cam.HLSDVRSeconds),
		boolToInt(cam.RecordingEnabled),
		cam.ID,
	)
	if err != nil {
		return fmt.Errorf("update camera: %w", err)
	}

	if _, err := tx.Exec(`DELETE FROM camera_motion WHERE camera_id=?`, cam.ID); err != nil {
		return err
	}
	if motion != nil {
		if err := insertMotion(tx, cam.ID, motion); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// UpdateCameraStreamInfo persists auto-detected stream values (codec, audio,
// resolution) back to the DB. Fields are only updated when the new value is
// non-zero/non-nil, so a failed probe doesn't overwrite explicit user config.
func UpdateCameraStreamInfo(database *DB, id, codec string, hasAudio *bool, width, height int) error {
	_, err := database.Exec(
		`UPDATE cameras
		 SET video_codec  = CASE WHEN ? != '' THEN ?  ELSE video_codec  END,
		     has_audio    = CASE WHEN ? IS NOT NULL THEN ? ELSE has_audio END,
		     width        = CASE WHEN ? != 0  THEN ?  ELSE width        END,
		     height       = CASE WHEN ? != 0  THEN ?  ELSE height       END
		 WHERE id = ?`,
		codec, codec,
		boolPtr(hasAudio), boolPtr(hasAudio),
		nullInt(width), nullInt(width),
		nullInt(height), nullInt(height),
		id,
	)
	return err
}

// DeleteCamera removes the camera (cascades to camera_motion).
func DeleteCamera(db *DB, id string) error {
	_, err := db.Exec(`DELETE FROM cameras WHERE id=?`, id)
	return err
}

// ReorderCameras sets display_order for each camera according to its position
// in ids (0-based). IDs not present in the list are left unchanged.
func ReorderCameras(database *DB, ids []string) error {
	tx, err := database.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	for i, id := range ids {
		if _, err := tx.Exec(`UPDATE cameras SET display_order=? WHERE id=?`, i, id); err != nil {
			return fmt.Errorf("reorder camera %q: %w", id, err)
		}
	}
	return tx.Commit()
}

// --- helpers ---

type execer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

const defaultPlaybackLeadSeconds = 10
const defaultPlaybackTrailSeconds = 10

func insertMotion(ex execer, cameraID string, m *config.MotionConfig) error {
	lead := m.PlaybackLeadSeconds
	if lead == 0 {
		lead = defaultPlaybackLeadSeconds
	}
	trail := m.PlaybackTrailSeconds
	if trail == 0 {
		trail = defaultPlaybackTrailSeconds
	}
	_, err := ex.Exec(
		`INSERT INTO camera_motion(camera_id, enabled, threshold, fps, cooldown_seconds, capture_width, capture_height, playback_lead_seconds, playback_trail_seconds)
		 VALUES(?,?,?,?,?,?,?,?,?)`,
		cameraID,
		boolToInt(m.Enabled),
		m.Threshold,
		m.FPS,
		m.CooldownSeconds,
		m.CaptureWidth,
		m.CaptureHeight,
		lead,
		trail,
	)
	return err
}

func getMotion(db *sql.DB, cameraID string) (*config.MotionConfig, error) {
	var m config.MotionConfig
	var enabled int
	err := db.QueryRow(
		`SELECT enabled, threshold, fps, cooldown_seconds, capture_width, capture_height, playback_lead_seconds, playback_trail_seconds FROM camera_motion WHERE camera_id=?`,
		cameraID,
	).Scan(&enabled, &m.Threshold, &m.FPS, &m.CooldownSeconds, &m.CaptureWidth, &m.CaptureHeight, &m.PlaybackLeadSeconds, &m.PlaybackTrailSeconds)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get motion for %q: %w", cameraID, err)
	}
	m.Enabled = enabled != 0
	return &m, nil
}

func durationToStr(d config.Duration, def time.Duration) string {
	if d == 0 {
		return formatDuration(def)
	}
	return formatDuration(time.Duration(d))
}

// formatDuration returns a clean duration string without trailing zero units.
// e.g. 5m0s → "5m", 1h0m0s → "1h", 30s → "30s".
func formatDuration(d time.Duration) string {
	if d == 0 {
		return "0s"
	}
	total := int64(d)
	if total%int64(time.Hour) == 0 {
		return fmt.Sprintf("%dh", total/int64(time.Hour))
	}
	if total%int64(time.Minute) == 0 {
		return fmt.Sprintf("%dm", total/int64(time.Minute))
	}
	if total%int64(time.Second) == 0 {
		return fmt.Sprintf("%ds", total/int64(time.Second))
	}
	return d.String()
}

func parseDuration(s string) config.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0
	}
	return config.Duration(d)
}

func nullStr(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

func nullInt(i int) sql.NullInt64 {
	return sql.NullInt64{Int64: int64(i), Valid: i != 0}
}

func boolPtr(b *bool) sql.NullInt64 {
	if b == nil {
		return sql.NullInt64{Valid: false}
	}
	if *b {
		return sql.NullInt64{Int64: 1, Valid: true}
	}
	return sql.NullInt64{Int64: 0, Valid: true}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func nullIntPtr(i *int) sql.NullInt64 {
	if i == nil {
		return sql.NullInt64{Valid: false}
	}
	return sql.NullInt64{Int64: int64(*i), Valid: true}
}

func scanIntPtr(n sql.NullInt64) *int {
	if !n.Valid {
		return nil
	}
	v := int(n.Int64)
	return &v
}
