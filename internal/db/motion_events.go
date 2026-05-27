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
	Color      string
	BboxX      float64
	BboxY      float64
	BboxW      float64
	BboxH      float64
}

// InsertMotionEvent adds a motion event row.
func InsertMotionEvent(db *DB, ev MotionEvent) error {
	_, err := db.Exec(`
		INSERT INTO motion_events(camera_id, occurred_at, score, frame_path, label, color, bbox_x, bbox_y, bbox_w, bbox_h)
		VALUES(?,?,?,?,?,?,?,?,?,?)`,
		ev.CameraID,
		ev.OccurredAt.UTC().Format(time.RFC3339),
		ev.Score,
		nullStr(ev.FramePath),
		nullStr(ev.Label),
		ev.Color,
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

// ListMotionEvents returns events for a camera in [start, end), ordered by occurred_at.
func ListMotionEvents(db *DB, cameraID string, start, end time.Time) ([]MotionEvent, error) {
	rows, err := db.Query(`
		SELECT id, camera_id, occurred_at, score, frame_path, label, color, bbox_x, bbox_y, bbox_w, bbox_h
		FROM motion_events
		WHERE camera_id=? AND occurred_at >= ? AND occurred_at < ?
		ORDER BY occurred_at`,
		cameraID,
		start.UTC().Format(time.RFC3339),
		end.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("list motion events: %w", err)
	}
	defer rows.Close()

	var events []MotionEvent
	for rows.Next() {
		var ev MotionEvent
		var occurredAt string
		var framePath, label sql.NullString
		var color string
		var bboxX, bboxY, bboxW, bboxH sql.NullFloat64
		if err := rows.Scan(&ev.ID, &ev.CameraID, &occurredAt, &ev.Score, &framePath, &label, &color, &bboxX, &bboxY, &bboxW, &bboxH); err != nil {
			return nil, fmt.Errorf("scan motion event: %w", err)
		}
		ev.OccurredAt, _ = time.Parse(time.RFC3339, occurredAt)
		ev.FramePath = framePath.String
		ev.Label = label.String
		ev.Color = color
		ev.BboxX = bboxX.Float64
		ev.BboxY = bboxY.Float64
		ev.BboxW = bboxW.Float64
		ev.BboxH = bboxH.Float64
		events = append(events, ev)
	}
	return events, rows.Err()
}

// GetMotionEventByID returns a single motion event by its primary key.
func GetMotionEventByID(db *DB, id int64) (MotionEvent, error) {
	var ev MotionEvent
	var occurredAt string
	var framePath, label sql.NullString
	var color string
	var bboxX, bboxY, bboxW, bboxH sql.NullFloat64
	err := db.QueryRow(`
		SELECT id, camera_id, occurred_at, score, frame_path, label, color, bbox_x, bbox_y, bbox_w, bbox_h
		FROM motion_events WHERE id=?`, id).
		Scan(&ev.ID, &ev.CameraID, &occurredAt, &ev.Score, &framePath, &label, &color, &bboxX, &bboxY, &bboxW, &bboxH)
	if err != nil {
		return MotionEvent{}, fmt.Errorf("get motion event: %w", err)
	}
	ev.OccurredAt, _ = time.Parse(time.RFC3339, occurredAt)
	ev.FramePath = framePath.String
	ev.Label = label.String
	ev.Color = color
	ev.BboxX = bboxX.Float64
	ev.BboxY = bboxY.Float64
	ev.BboxW = bboxW.Float64
	ev.BboxH = bboxH.Float64
	return ev, nil
}

// MinMaxScoreForDay returns the min and max motion score for a camera in [start, end).
// Returns 0, 0 when there are no events.
func MinMaxScoreForDay(db *DB, cameraID string, start, end time.Time) (min, max float64, err error) {
	var mn, mx sql.NullFloat64
	err = db.QueryRow(`
		SELECT MIN(score), MAX(score)
		FROM motion_events
		WHERE camera_id=? AND occurred_at >= ? AND occurred_at < ?`,
		cameraID,
		start.UTC().Format(time.RFC3339),
		end.UTC().Format(time.RFC3339),
	).Scan(&mn, &mx)
	if err != nil {
		return 0, 0, fmt.Errorf("min/max score: %w", err)
	}
	return mn.Float64, mx.Float64, nil
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
