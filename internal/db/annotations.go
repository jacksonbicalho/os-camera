package db

import (
	"database/sql"
	"time"
)

type Annotation struct {
	ID          int64     `json:"id"`
	EventID     int64     `json:"event_id"`
	Label       string    `json:"label"`
	BboxX       float64   `json:"bbox_x"`
	BboxY       float64   `json:"bbox_y"`
	BboxW       float64   `json:"bbox_w"`
	BboxH       float64   `json:"bbox_h"`
	RotationDeg float64   `json:"rotation_deg"`
	CreatedAt   time.Time `json:"created_at"`
}

func InsertAnnotation(d *DB, a Annotation) (int64, error) {
	res, err := d.Exec(`
		INSERT INTO annotations (event_id, label, bbox_x, bbox_y, bbox_w, bbox_h, rotation_deg)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		a.EventID, a.Label, a.BboxX, a.BboxY, a.BboxW, a.BboxH, a.RotationDeg)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func ListAnnotationsByEvent(d *DB, eventID int64) ([]Annotation, error) {
	rows, err := d.Query(`
		SELECT id, event_id, label, bbox_x, bbox_y, bbox_w, bbox_h, rotation_deg, created_at
		FROM annotations WHERE event_id=? ORDER BY id`, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []Annotation
	for rows.Next() {
		var a Annotation
		var createdAt string
		if err := rows.Scan(&a.ID, &a.EventID, &a.Label, &a.BboxX, &a.BboxY, &a.BboxW, &a.BboxH, &a.RotationDeg, &createdAt); err != nil {
			return nil, err
		}
		a.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		result = append(result, a)
	}
	return result, rows.Err()
}

func UpdateAnnotation(d *DB, id int64, a Annotation) error {
	_, err := d.Exec(`
		UPDATE annotations SET label=?, bbox_x=?, bbox_y=?, bbox_w=?, bbox_h=?, rotation_deg=?
		WHERE id=?`,
		a.Label, a.BboxX, a.BboxY, a.BboxW, a.BboxH, a.RotationDeg, id)
	return err
}

func DeleteAnnotation(d *DB, id int64) error {
	_, err := d.Exec(`DELETE FROM annotations WHERE id=?`, id)
	return err
}

func CountAnnotations(d *DB) (int, error) {
	var n int
	err := d.QueryRow(`SELECT COUNT(*) FROM annotations`).Scan(&n)
	return n, err
}

// CountLabeledEvents returns the number of motion_events with a non-empty label.
func CountLabeledEvents(d *DB) (int, error) {
	var n int
	err := d.QueryRow(`SELECT COUNT(*) FROM motion_events WHERE label IS NOT NULL AND label != ''`).Scan(&n)
	return n, err
}

// ListLabeledEvents returns motion events that have a non-empty label and a frame_path.
// Events that already have a bounding-box annotation are excluded to avoid duplicates.
func ListLabeledEvents(d *DB) ([]MotionEvent, error) {
	rows, err := d.Query(`
		SELECT id, camera_id, occurred_at, score, frame_path, label, color, bbox_x, bbox_y, bbox_w, bbox_h
		FROM motion_events
		WHERE label IS NOT NULL AND label != ''
		  AND frame_path IS NOT NULL AND frame_path != ''
		  AND id NOT IN (SELECT DISTINCT event_id FROM annotations)
		ORDER BY id`)
	if err != nil {
		return nil, err
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
			return nil, err
		}
		ev.OccurredAt, _ = time.Parse(time.RFC3339, occurredAt)
		ev.FramePath = framePath.String
		ev.Label = label.String
		events = append(events, ev)
	}
	return events, rows.Err()
}

func ListAllAnnotations(d *DB) ([]Annotation, error) {
	rows, err := d.Query(`
		SELECT id, event_id, label, bbox_x, bbox_y, bbox_w, bbox_h, rotation_deg, created_at
		FROM annotations ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []Annotation
	for rows.Next() {
		var a Annotation
		var createdAt string
		if err := rows.Scan(&a.ID, &a.EventID, &a.Label, &a.BboxX, &a.BboxY, &a.BboxW, &a.BboxH, &a.RotationDeg, &createdAt); err != nil {
			return nil, err
		}
		a.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		result = append(result, a)
	}
	return result, rows.Err()
}
