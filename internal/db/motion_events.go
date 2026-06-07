package db

import (
	"database/sql"
	"fmt"
	"strings"
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
	Dismissed  bool
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
		SELECT id, camera_id, occurred_at, score, frame_path, label, color, bbox_x, bbox_y, bbox_w, bbox_h, dismissed
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
		ev, err := scanMotionEvent(rows)
		if err != nil {
			return nil, fmt.Errorf("scan motion event: %w", err)
		}
		events = append(events, ev)
	}
	return events, rows.Err()
}

// ListOrphanedMotionEvents returns motion events older than cutoff whose
// occurred_at is not covered by any recording. Used by the cleaner to purge
// events left behind when their recording was already removed (or never
// existed). Events within retention or covered by a recording are excluded.
// occurred_at and recordings times are both RFC3339 text, so the comparisons
// are plain lexicographic ordering.
func ListOrphanedMotionEvents(db *DB, cutoff time.Time) ([]MotionEvent, error) {
	rows, err := db.Query(`
		SELECT id, camera_id, occurred_at, score, frame_path, label, color, bbox_x, bbox_y, bbox_w, bbox_h, dismissed
		FROM motion_events me
		WHERE occurred_at < ?
		  AND NOT EXISTS (
		    SELECT 1 FROM recordings r
		    WHERE r.camera_id = me.camera_id
		      AND r.started_at <= me.occurred_at
		      AND (r.ended_at IS NULL OR r.ended_at >= me.occurred_at)
		  )
		ORDER BY occurred_at`,
		cutoff.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("list orphaned motion events: %w", err)
	}
	defer rows.Close()

	var events []MotionEvent
	for rows.Next() {
		ev, err := scanMotionEvent(rows)
		if err != nil {
			return nil, fmt.Errorf("scan motion event: %w", err)
		}
		events = append(events, ev)
	}
	return events, rows.Err()
}

// GetMotionEventByID returns a single motion event by its primary key.
func GetMotionEventByID(db *DB, id int64) (MotionEvent, error) {
	row := db.QueryRow(`
		SELECT id, camera_id, occurred_at, score, frame_path, label, color, bbox_x, bbox_y, bbox_w, bbox_h, dismissed
		FROM motion_events WHERE id=?`, id)
	ev, err := scanMotionEvent(row)
	if err != nil {
		return MotionEvent{}, fmt.Errorf("get motion event: %w", err)
	}
	return ev, nil
}

// scanner abstracts sql.Row and sql.Rows for scanMotionEvent.
type scanner interface {
	Scan(dest ...any) error
}

func scanMotionEvent(s scanner) (MotionEvent, error) {
	var ev MotionEvent
	var occurredAt string
	var framePath, label sql.NullString
	var color string
	var bboxX, bboxY, bboxW, bboxH sql.NullFloat64
	var dismissed int
	if err := s.Scan(&ev.ID, &ev.CameraID, &occurredAt, &ev.Score, &framePath, &label, &color, &bboxX, &bboxY, &bboxW, &bboxH, &dismissed); err != nil {
		return MotionEvent{}, err
	}
	ev.OccurredAt, _ = time.Parse(time.RFC3339, occurredAt)
	ev.FramePath = framePath.String
	ev.Label = label.String
	ev.Color = color
	ev.BboxX = bboxX.Float64
	ev.BboxY = bboxY.Float64
	ev.BboxW = bboxW.Float64
	ev.BboxH = bboxH.Float64
	ev.Dismissed = dismissed != 0
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

// UpdateMotionEventFramePath replaces the frame_path of a motion event.
// Returns an error if no row with the given id exists.
func UpdateMotionEventFramePath(db *DB, id int64, framePath string) error {
	res, err := db.Exec(`UPDATE motion_events SET frame_path=? WHERE id=?`, framePath, id)
	if err != nil {
		return fmt.Errorf("update motion event frame_path: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("motion event %d not found", id)
	}
	return nil
}

// UpdateMotionEventLabel sets or clears the label of a motion event.
func UpdateMotionEventLabel(db *DB, id int64, label string) error {
	_, err := db.Exec(`UPDATE motion_events SET label=? WHERE id=?`, nullStr(label), id)
	if err != nil {
		return fmt.Errorf("update motion event label: %w", err)
	}
	return nil
}

// BulkDeleteMotionEvents deletes the motion events with the given IDs and
// returns the affected count plus the snapshot of every deleted row (with
// camera_id, occurred_at and frame_path) so the caller can resolve and remove
// the JPEGs from disk. Empty ids returns (0, nil, nil).
func BulkDeleteMotionEvents(db *DB, ids []int64) (int64, []MotionEvent, error) {
	if len(ids) == 0 {
		return 0, nil, nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	in := strings.Join(placeholders, ",")

	rows, err := db.Query(`SELECT id, camera_id, occurred_at, frame_path FROM motion_events WHERE id IN (`+in+`)`, args...)
	if err != nil {
		return 0, nil, fmt.Errorf("select events: %w", err)
	}
	var events []MotionEvent
	for rows.Next() {
		var ev MotionEvent
		var occurred string
		var fp sql.NullString
		if err := rows.Scan(&ev.ID, &ev.CameraID, &occurred, &fp); err != nil {
			rows.Close()
			return 0, nil, fmt.Errorf("scan event: %w", err)
		}
		ev.OccurredAt, _ = time.Parse(time.RFC3339, occurred)
		ev.FramePath = fp.String
		events = append(events, ev)
	}
	rows.Close()

	res, err := db.Exec(`DELETE FROM motion_events WHERE id IN (`+in+`)`, args...)
	if err != nil {
		return 0, nil, fmt.Errorf("delete motion events: %w", err)
	}
	deleted, _ := res.RowsAffected()
	return deleted, events, nil
}

// BulkUpdateMotionEventLabels sets (or clears, when label is empty) the label
// for the given motion event IDs. Returns the affected row count.
func BulkUpdateMotionEventLabels(db *DB, ids []int64, label string) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, 0, len(ids)+1)
	args = append(args, nullStr(label))
	for i, id := range ids {
		placeholders[i] = "?"
		args = append(args, id)
	}
	in := strings.Join(placeholders, ",")
	res, err := db.Exec(`UPDATE motion_events SET label=? WHERE id IN (`+in+`)`, args...)
	if err != nil {
		return 0, fmt.Errorf("bulk update labels: %w", err)
	}
	updated, _ := res.RowsAffected()
	return updated, nil
}

// PageMotionEvents returns a page of motion events for a camera, ordered by occurred_at DESC.
// offset and limit control pagination; unlabeledOnly filters to events with no label;
// labelSearch filters to events whose label contains the given string (case-insensitive, ignored when empty).
// dismissedOnly=true returns only dismissed events; false (default) excludes them.
// Returns the events, the total matching count, and any error.
func PageMotionEvents(db *DB, cameraID string, offset, limit int, unlabeledOnly bool, labelSearch string, dismissedOnly bool) ([]MotionEvent, int, error) {
	filter := ""
	args := []any{cameraID}
	if dismissedOnly {
		filter += " AND dismissed=1"
	} else {
		filter += " AND dismissed=0"
	}
	if unlabeledOnly {
		filter += " AND (label IS NULL OR label = '')"
	} else if labelSearch != "" {
		filter += " AND label LIKE ? COLLATE NOCASE"
		args = append(args, "%"+labelSearch+"%")
	}

	countArgs := args
	var total int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM motion_events WHERE camera_id=?`+filter, countArgs...,
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count motion events: %w", err)
	}

	queryArgs := append(args, limit, offset)
	rows, err := db.Query(
		`SELECT id, camera_id, occurred_at, score, frame_path, label, color, bbox_x, bbox_y, bbox_w, bbox_h, dismissed
		 FROM motion_events WHERE camera_id=?`+filter+`
		 ORDER BY occurred_at DESC LIMIT ? OFFSET ?`,
		queryArgs...,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("page motion events: %w", err)
	}
	defer rows.Close()

	var events []MotionEvent
	for rows.Next() {
		ev, err := scanMotionEvent(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan motion event: %w", err)
		}
		events = append(events, ev)
	}
	return events, total, rows.Err()
}

// BulkDismissMotionEvents marks the given motion event IDs as dismissed=1.
// Empty ids returns (0, nil).
func BulkDismissMotionEvents(db *DB, ids []int64) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	in := strings.Join(placeholders, ",")
	res, err := db.Exec(`UPDATE motion_events SET dismissed=1 WHERE id IN (`+in+`)`, args...)
	if err != nil {
		return 0, fmt.Errorf("dismiss motion events: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

func nullFloat(f float64) sql.NullFloat64 {
	return sql.NullFloat64{Float64: f, Valid: f != 0}
}
