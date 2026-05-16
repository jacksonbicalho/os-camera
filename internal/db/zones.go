package db

import (
	"database/sql"
	"fmt"

	"camera/internal/zones"
)

// GetZones returns all zones for the given camera, ordered by display_order.
func GetZones(database *DB, cameraID string) ([]zones.Zone, error) {
	rows, err := database.Query(
		`SELECT x, y, w, h, type, label, threshold, cooldown_seconds, fps, scale, color
		 FROM camera_motion_zones
		 WHERE camera_id = ?
		 ORDER BY display_order, id`,
		cameraID,
	)
	if err != nil {
		return nil, fmt.Errorf("get zones for %q: %w", cameraID, err)
	}
	defer rows.Close()

	var zs []zones.Zone
	for rows.Next() {
		var z zones.Zone
		var label sql.NullString
		if err := rows.Scan(&z.X, &z.Y, &z.W, &z.H, &z.Type, &label, &z.Threshold, &z.CooldownSeconds, &z.FPS, &z.Scale, &z.Color); err != nil {
			return nil, fmt.Errorf("scan zone: %w", err)
		}
		z.Label = label.String
		zs = append(zs, z)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if zs == nil {
		zs = []zones.Zone{}
	}
	return zs, nil
}

// SetZones replaces all zones for the given camera in a single transaction.
func SetZones(database *DB, cameraID string, zs []zones.Zone) error {
	tx, err := database.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.Exec(`DELETE FROM camera_motion_zones WHERE camera_id = ?`, cameraID); err != nil {
		return fmt.Errorf("delete zones: %w", err)
	}

	for i, z := range zs {
		zType := z.Type
		if zType == "" {
			zType = "exclude"
		}
		_, err := tx.Exec(
			`INSERT INTO camera_motion_zones
			 (camera_id, display_order, x, y, w, h, type, label, threshold, cooldown_seconds, fps, scale, color)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			cameraID, i, z.X, z.Y, z.W, z.H, zType, nullStr(z.Label), z.Threshold, z.CooldownSeconds, z.FPS, z.Scale, z.Color,
		)
		if err != nil {
			return fmt.Errorf("insert zone %d: %w", i, err)
		}
	}

	return tx.Commit()
}
