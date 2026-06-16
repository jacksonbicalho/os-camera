package db

import (
	"database/sql"
	"fmt"

	"camera/internal/stateclass"
)

// loadClasses returns the ordered class labels of a classifier.
func loadClasses(q interface {
	Query(string, ...any) (*sql.Rows, error)
}, classifierID int64) ([]string, error) {
	rows, err := q.Query(
		`SELECT label FROM camera_state_classes WHERE classifier_id = ? ORDER BY display_order, label`,
		classifierID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	classes := []string{}
	for rows.Next() {
		var l string
		if err := rows.Scan(&l); err != nil {
			return nil, err
		}
		classes = append(classes, l)
	}
	return classes, rows.Err()
}

func scanClassifier(s interface{ Scan(...any) error }) (stateclass.Classifier, error) {
	var c stateclass.Classifier
	var triggerMotion, enabled int
	err := s.Scan(
		&c.ID, &c.CameraID, &c.Name, &c.Model, &c.Threshold,
		&triggerMotion, &c.TriggerIntervalSeconds,
		&c.CropX, &c.CropY, &c.CropW, &c.CropH,
		&c.MinConsecutive, &enabled,
	)
	c.TriggerMotion = triggerMotion != 0
	c.Enabled = enabled != 0
	return c, err
}

const classifierCols = `id, camera_id, name, model, threshold,
	trigger_motion, trigger_interval_seconds,
	crop_x, crop_y, crop_w, crop_h, min_consecutive, enabled`

// CreateStateClassifier inserts a classifier and its classes, returning the new id.
func CreateStateClassifier(database *DB, c stateclass.Classifier) (int64, error) {
	tx, err := database.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback() //nolint:errcheck

	res, err := tx.Exec(
		`INSERT INTO camera_state_classifiers
		 (camera_id, name, model, threshold, trigger_motion, trigger_interval_seconds,
		  crop_x, crop_y, crop_w, crop_h, min_consecutive, enabled)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.CameraID, c.Name, c.Model, c.Threshold, boolToInt(c.TriggerMotion), c.TriggerIntervalSeconds,
		c.CropX, c.CropY, c.CropW, c.CropH, c.MinConsecutive, boolToInt(c.Enabled),
	)
	if err != nil {
		return 0, fmt.Errorf("insert classifier: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if err := insertClasses(tx, id, c.Classes); err != nil {
		return 0, err
	}
	return id, tx.Commit()
}

func insertClasses(tx *sql.Tx, classifierID int64, classes []string) error {
	for i, label := range classes {
		if _, err := tx.Exec(
			`INSERT INTO camera_state_classes (classifier_id, label, display_order) VALUES (?, ?, ?)`,
			classifierID, label, i,
		); err != nil {
			return fmt.Errorf("insert class %q: %w", label, err)
		}
	}
	return nil
}

// ListStateClassifiers returns the classifiers of a camera (with their classes).
func ListStateClassifiers(database *DB, cameraID string) ([]stateclass.Classifier, error) {
	rows, err := database.Query(
		`SELECT `+classifierCols+` FROM camera_state_classifiers WHERE camera_id = ? ORDER BY id`,
		cameraID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []stateclass.Classifier{}
	for rows.Next() {
		c, err := scanClassifier(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		classes, err := loadClasses(database, out[i].ID)
		if err != nil {
			return nil, err
		}
		out[i].Classes = classes
	}
	return out, nil
}

// GetStateClassifier returns one classifier (with classes), or sql.ErrNoRows.
func GetStateClassifier(database *DB, id int64) (stateclass.Classifier, error) {
	row := database.QueryRow(
		`SELECT `+classifierCols+` FROM camera_state_classifiers WHERE id = ?`, id,
	)
	c, err := scanClassifier(row)
	if err != nil {
		return c, err
	}
	c.Classes, err = loadClasses(database, id)
	return c, err
}

// UpdateStateClassifier updates a classifier and replaces its classes.
func UpdateStateClassifier(database *DB, c stateclass.Classifier) error {
	tx, err := database.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.Exec(
		`UPDATE camera_state_classifiers SET
		   name = ?, model = ?, threshold = ?, trigger_motion = ?, trigger_interval_seconds = ?,
		   crop_x = ?, crop_y = ?, crop_w = ?, crop_h = ?, min_consecutive = ?, enabled = ?
		 WHERE id = ?`,
		c.Name, c.Model, c.Threshold, boolToInt(c.TriggerMotion), c.TriggerIntervalSeconds,
		c.CropX, c.CropY, c.CropW, c.CropH, c.MinConsecutive, boolToInt(c.Enabled), c.ID,
	); err != nil {
		return fmt.Errorf("update classifier: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM camera_state_classes WHERE classifier_id = ?`, c.ID); err != nil {
		return err
	}
	if err := insertClasses(tx, c.ID, c.Classes); err != nil {
		return err
	}
	return tx.Commit()
}

// DeleteStateClassifier removes a classifier (classes/history cascade).
func DeleteStateClassifier(database *DB, id int64) error {
	_, err := database.Exec(`DELETE FROM camera_state_classifiers WHERE id = ?`, id)
	return err
}

// RecordStateTransition appends a confirmed state to a classifier's history.
func RecordStateTransition(database *DB, classifierID int64, state string, confidence float64) error {
	_, err := database.Exec(
		`INSERT INTO camera_state_history (classifier_id, state, confidence) VALUES (?, ?, ?)`,
		classifierID, state, confidence,
	)
	return err
}

// GetCurrentState returns the latest state of a classifier, or nil when none yet.
func GetCurrentState(database *DB, classifierID int64) (*stateclass.State, error) {
	row := database.QueryRow(
		`SELECT state, confidence, changed_at FROM camera_state_history
		 WHERE classifier_id = ? ORDER BY changed_at DESC, id DESC LIMIT 1`,
		classifierID,
	)
	var st stateclass.State
	err := row.Scan(&st.State, &st.Confidence, &st.ChangedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &st, nil
}
