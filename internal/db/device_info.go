package db

import (
	"fmt"
	"time"
)

// SaveDeviceInfo replaces a camera's captured device-info snapshot with the
// given key/value pairs (a full snapshot: stale keys are removed). All rows
// share one collected_at timestamp.
func SaveDeviceInfo(database *DB, cameraID string, values map[string]string) error {
	tx, err := database.Begin()
	if err != nil {
		return fmt.Errorf("save device info: begin: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM camera_device_info WHERE camera_id=?`, cameraID); err != nil {
		return fmt.Errorf("save device info: clear: %w", err)
	}

	stmt, err := tx.Prepare(
		`INSERT INTO camera_device_info (camera_id, key, value, collected_at) VALUES (?,?,?,?)`)
	if err != nil {
		return fmt.Errorf("save device info: prepare: %w", err)
	}
	defer stmt.Close()

	now := time.Now().UTC()
	for k, v := range values {
		if _, err := stmt.Exec(cameraID, k, v, now); err != nil {
			return fmt.Errorf("save device info: insert %q: %w", k, err)
		}
	}
	return tx.Commit()
}

// GetDeviceInfo returns a camera's captured device-info snapshot and when it was
// collected. ok is false when nothing has been captured yet.
func GetDeviceInfo(database *DB, cameraID string) (values map[string]string, collectedAt time.Time, ok bool, err error) {
	rows, err := database.Query(
		`SELECT key, value, collected_at FROM camera_device_info WHERE camera_id=?`, cameraID)
	if err != nil {
		return nil, time.Time{}, false, fmt.Errorf("get device info: %w", err)
	}
	defer rows.Close()

	values = map[string]string{}
	for rows.Next() {
		var k, v string
		var ts time.Time
		if err := rows.Scan(&k, &v, &ts); err != nil {
			return nil, time.Time{}, false, fmt.Errorf("get device info: scan: %w", err)
		}
		values[k] = v
		collectedAt = ts
	}
	if err := rows.Err(); err != nil {
		return nil, time.Time{}, false, fmt.Errorf("get device info: rows: %w", err)
	}
	if len(values) == 0 {
		return nil, time.Time{}, false, nil
	}
	return values, collectedAt, true, nil
}
