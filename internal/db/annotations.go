package db

import "time"

type Annotation struct {
	ID        int64     `json:"id"`
	EventID   int64     `json:"event_id"`
	Label     string    `json:"label"`
	BboxX     float64   `json:"bbox_x"`
	BboxY     float64   `json:"bbox_y"`
	BboxW     float64   `json:"bbox_w"`
	BboxH     float64   `json:"bbox_h"`
	CreatedAt time.Time `json:"created_at"`
}

func InsertAnnotation(d *DB, a Annotation) (int64, error) {
	res, err := d.Exec(`
		INSERT INTO annotations (event_id, label, bbox_x, bbox_y, bbox_w, bbox_h)
		VALUES (?, ?, ?, ?, ?, ?)`,
		a.EventID, a.Label, a.BboxX, a.BboxY, a.BboxW, a.BboxH)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func ListAnnotationsByEvent(d *DB, eventID int64) ([]Annotation, error) {
	rows, err := d.Query(`
		SELECT id, event_id, label, bbox_x, bbox_y, bbox_w, bbox_h, created_at
		FROM annotations WHERE event_id=? ORDER BY id`, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []Annotation
	for rows.Next() {
		var a Annotation
		var createdAt string
		if err := rows.Scan(&a.ID, &a.EventID, &a.Label, &a.BboxX, &a.BboxY, &a.BboxW, &a.BboxH, &createdAt); err != nil {
			return nil, err
		}
		a.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		result = append(result, a)
	}
	return result, rows.Err()
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

func ListAllAnnotations(d *DB) ([]Annotation, error) {
	rows, err := d.Query(`
		SELECT id, event_id, label, bbox_x, bbox_y, bbox_w, bbox_h, created_at
		FROM annotations ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []Annotation
	for rows.Next() {
		var a Annotation
		var createdAt string
		if err := rows.Scan(&a.ID, &a.EventID, &a.Label, &a.BboxX, &a.BboxY, &a.BboxW, &a.BboxH, &createdAt); err != nil {
			return nil, err
		}
		a.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		result = append(result, a)
	}
	return result, rows.Err()
}
