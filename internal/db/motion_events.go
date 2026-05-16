package db

import (
	"database/sql"
	"fmt"
	"time"
)

// MotionEvent represents a row in the motion_events table.
type MotionEvent struct {
	ID         int64
	CameraID   string
	OccurredAt time.Time
	Score      float64
	FramePath  string
	Label      string
	BboxX      float64
	BboxY      float64
	BboxW      float64
	BboxH      float64
}

// InsertMotionEvent adds a motion event row.
func InsertMotionEvent(db *DB, ev MotionEvent) error {
	_, err := db.Exec(`
		INSERT INTO motion_events(camera_id, occurred_at, score, frame_path, label, bbox_x, bbox_y, bbox_w, bbox_h)
		VALUES(?,?,?,?,?,?,?,?,?)`,
		ev.CameraID,
		ev.OccurredAt.UTC().Format(time.RFC3339),
		ev.Score,
		nullStr(ev.FramePath),
		nullStr(ev.Label),
		nullFloat(ev.BboxX),
		nullFloat(ev.BboxY),
		nullFloat(ev.BboxW),
		nullFloat(ev.BboxH),
	)
	if err != nil {
		return fmt.Errorf("insert motion event: %w", err)
	}
	return nil
}

// CountMotionEvents returns the total number of motion events recorded for a camera.
func CountMotionEvents(db *DB, cameraID string) (int64, error) {
	var n int64
	err := db.QueryRow(`SELECT COUNT(*) FROM motion_events WHERE camera_id=?`, cameraID).Scan(&n)
	return n, err
}

// DeleteMotionEventsInRange removes motion events for a camera in [start, end).
func DeleteMotionEventsInRange(db *DB, cameraID string, start, end time.Time) error {
	_, err := db.Exec(`
		DELETE FROM motion_events
		WHERE camera_id=?
		  AND occurred_at >= ?
		  AND occurred_at < ?`,
		cameraID,
		start.UTC().Format(time.RFC3339),
		end.UTC().Format(time.RFC3339),
	)
	return err
}

func nullFloat(f float64) sql.NullFloat64 {
	return sql.NullFloat64{Float64: f, Valid: f != 0}
}
