package db

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"camera/internal/config"
)

// CreateCamera inserts a camera row and, if motion is non-nil, its
// camera_motion row.
func CreateCamera(db *DB, cam config.CameraConfig, motion *config.MotionConfig) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	_, err = tx.Exec(
		`INSERT INTO cameras(id, rtsp_url, chunk_duration, reconnect_interval,
		                     video_codec, has_audio, width, height, display_order)
		 VALUES(?,?,?,?,?,?,?,?,?)`,
		cam.ID,
		cam.RTSPURL,
		durationToStr(cam.ChunkDuration, config.DefaultChunkDuration),
		durationToStr(cam.ReconnectInterval, config.DefaultReconnectInterval),
		nullStr(cam.VideoCodec),
		boolPtr(cam.HasAudio),
		nullInt(cam.Width),
		nullInt(cam.Height),
		cam.DisplayOrder,
	)
	if err != nil {
		return fmt.Errorf("insert camera: %w", err)
	}

	if motion != nil {
		if err := insertMotion(tx, cam.ID, motion); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetCamera returns the camera with the given ID, including its motion config
// if present.
func GetCamera(db *DB, id string) (config.CameraConfig, error) {
	var cam config.CameraConfig
	var chunk, reconnect string
	var codec sql.NullString
	var hasAudio sql.NullInt64
	var width, height sql.NullInt64

	err := db.QueryRow(
		`SELECT id, rtsp_url, chunk_duration, reconnect_interval,
		        video_codec, has_audio, width, height, display_order
		 FROM cameras WHERE id=?`, id,
	).Scan(
		&cam.ID, &cam.RTSPURL, &chunk, &reconnect,
		&codec, &hasAudio, &width, &height, &cam.DisplayOrder,
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
		SELECT c.id, c.rtsp_url, c.chunk_duration, c.reconnect_interval,
		       c.video_codec, c.has_audio, c.width, c.height, c.display_order,
		       cm.enabled, cm.threshold, cm.fps, cm.cooldown_seconds
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
		var hasAudio, width, height sql.NullInt64
		var mEnabled sql.NullInt64
		var mThreshold sql.NullFloat64
		var mFPS, mCooldown sql.NullInt64

		if err := rows.Scan(
			&cam.ID, &cam.RTSPURL, &chunk, &reconnect,
			&codec, &hasAudio, &width, &height, &cam.DisplayOrder,
			&mEnabled, &mThreshold, &mFPS, &mCooldown,
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
		if mEnabled.Valid {
			cam.Motion = &config.MotionConfig{
				Enabled:         mEnabled.Int64 != 0,
				Threshold:       mThreshold.Float64,
				FPS:             int(mFPS.Int64),
				CooldownSeconds: int(mCooldown.Int64),
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
		                    video_codec=?, has_audio=?, width=?, height=?, display_order=?
		 WHERE id=?`,
		cam.RTSPURL,
		durationToStr(cam.ChunkDuration, config.DefaultChunkDuration),
		durationToStr(cam.ReconnectInterval, config.DefaultReconnectInterval),
		nullStr(cam.VideoCodec),
		boolPtr(cam.HasAudio),
		nullInt(cam.Width),
		nullInt(cam.Height),
		cam.DisplayOrder,
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

// --- helpers ---

type execer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

func insertMotion(ex execer, cameraID string, m *config.MotionConfig) error {
	_, err := ex.Exec(
		`INSERT INTO camera_motion(camera_id, enabled, threshold, fps, cooldown_seconds)
		 VALUES(?,?,?,?,?)`,
		cameraID,
		boolToInt(m.Enabled),
		m.Threshold,
		m.FPS,
		m.CooldownSeconds,
	)
	return err
}

func getMotion(db *sql.DB, cameraID string) (*config.MotionConfig, error) {
	var m config.MotionConfig
	var enabled int
	err := db.QueryRow(
		`SELECT enabled, threshold, fps, cooldown_seconds FROM camera_motion WHERE camera_id=?`,
		cameraID,
	).Scan(&enabled, &m.Threshold, &m.FPS, &m.CooldownSeconds)
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
		return def.String()
	}
	return time.Duration(d).String()
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
