package db

import (
	"database/sql"
	"strings"
	"time"
)

// Recording represents a row in the recordings table.
type Recording struct {
	ID        int64
	CameraID  string
	StartedAt time.Time
	EndedAt   time.Time
	Path      string
	SizeBytes int64
	HasMotion bool
}

// InsertRecording adds a recording row. If a row with the same path already
// exists the insert is silently skipped (idempotent backfill).
func InsertRecording(db *DB, r Recording) error {
	_, err := db.Exec(`
		INSERT OR IGNORE INTO recordings(camera_id, started_at, ended_at, path, size_bytes, has_motion)
		VALUES(?,?,?,?,?,?)`,
		r.CameraID,
		r.StartedAt.UTC().Format(time.RFC3339),
		nullTime(r.EndedAt),
		r.Path,
		r.SizeBytes,
		boolToInt(r.HasMotion),
	)
	return err
}

// MarkRecordingHasMotion sets has_motion=true for all recordings of a camera
// whose time range overlaps [start, end).
func MarkRecordingHasMotion(db *DB, cameraID string, start, end time.Time) error {
	_, err := db.Exec(`
		UPDATE recordings SET has_motion=1
		WHERE camera_id=?
		  AND started_at < ?
		  AND (ended_at IS NULL OR ended_at > ?)`,
		cameraID,
		end.UTC().Format(time.RFC3339),
		start.UTC().Format(time.RFC3339),
	)
	return err
}

// HasMotionInRangeDB reports whether the recordings table contains a row with
// has_motion=1 overlapping [start, end) for the given camera. Returns false
// when the DB has no rows for this range (caller may fall back to NDJSON).
func HasMotionInRangeDB(db *DB, cameraID string, start, end time.Time) (found bool, hasMotion bool, err error) {
	var count int
	var motion int
	err = db.QueryRow(`
		SELECT COUNT(*), COALESCE(MAX(has_motion),0)
		FROM recordings
		WHERE camera_id=?
		  AND started_at >= ?
		  AND started_at < ?`,
		cameraID,
		start.UTC().Format(time.RFC3339),
		end.UTC().Format(time.RFC3339),
	).Scan(&count, &motion)
	if err != nil {
		return false, false, err
	}
	return count > 0, motion != 0, nil
}

// IDsByPaths returns a map of path → recording ID for the given file paths.
// Paths not found in the DB are absent from the map.
func IDsByPaths(db *DB, paths []string) (map[string]int64, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	placeholders := strings.Repeat("?,", len(paths))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, len(paths))
	for i, p := range paths {
		args[i] = p
	}
	rows, err := db.Query(`SELECT id, path FROM recordings WHERE path IN (`+placeholders+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string]int64, len(paths))
	for rows.Next() {
		var id int64
		var path string
		if err := rows.Scan(&id, &path); err != nil {
			return nil, err
		}
		result[path] = id
	}
	return result, rows.Err()
}

// GetRecordingByID returns the recording row for the given ID, or an error if not found.
func GetRecordingByID(database *DB, id int64) (*Recording, error) {
	var r Recording
	var startedAt, endedAt string
	err := database.QueryRow(
		`SELECT id, camera_id, started_at, COALESCE(ended_at,''), path, size_bytes, has_motion
		 FROM recordings WHERE id=?`, id,
	).Scan(&r.ID, &r.CameraID, &startedAt, &endedAt, &r.Path, &r.SizeBytes, &r.HasMotion)
	if err != nil {
		return nil, err
	}
	r.StartedAt, err = time.Parse(time.RFC3339, startedAt)
	if err != nil {
		return nil, err
	}
	if endedAt != "" {
		r.EndedAt, _ = time.Parse(time.RFC3339, endedAt)
	}
	return &r, nil
}

// HasMotionByPaths returns a map of path → has_motion for the given file paths.
// Paths not found in the DB are absent from the map (caller may treat as false).
func HasMotionByPaths(db *DB, paths []string) (map[string]bool, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	placeholders := strings.Repeat("?,", len(paths))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, len(paths))
	for i, p := range paths {
		args[i] = p
	}
	rows, err := db.Query(`SELECT path, has_motion FROM recordings WHERE path IN (`+placeholders+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string]bool, len(paths))
	for rows.Next() {
		var path string
		var hm int
		if err := rows.Scan(&path, &hm); err != nil {
			return nil, err
		}
		result[path] = hm != 0
	}
	return result, rows.Err()
}

// DeleteRecording removes the recording row for the given path.
func DeleteRecording(db *DB, path string) error {
	_, err := db.Exec(`DELETE FROM recordings WHERE path=?`, path)
	return err
}

// EndedAtByStartedAt returns the ended_at for a recording identified by camera and started_at.
// Returns a zero time if the row does not exist or ended_at is NULL.
func EndedAtByStartedAt(db *DB, cameraID string, startedAt time.Time) (time.Time, error) {
	var s sql.NullString
	err := db.QueryRow(`SELECT ended_at FROM recordings WHERE camera_id=? AND started_at=?`,
		cameraID, startedAt.UTC().Format(time.RFC3339),
	).Scan(&s)
	if err != nil {
		return time.Time{}, err
	}
	if !s.Valid || s.String == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, s.String)
}

// DeleteRecordingByStartedAt removes the recording row for a camera that started at the given time.
func DeleteRecordingByStartedAt(db *DB, cameraID string, startedAt time.Time) error {
	_, err := db.Exec(`DELETE FROM recordings WHERE camera_id=? AND started_at=?`,
		cameraID, startedAt.UTC().Format(time.RFC3339))
	return err
}

// SizeByCamera returns the total size in bytes per camera, ordered by camera_id.
func SizeByCamera(db *DB) (map[string]int64, error) {
	rows, err := db.Query(`SELECT camera_id, SUM(size_bytes) FROM recordings GROUP BY camera_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string]int64)
	for rows.Next() {
		var cam string
		var total sql.NullInt64
		if err := rows.Scan(&cam, &total); err != nil {
			return nil, err
		}
		result[cam] = total.Int64
	}
	return result, rows.Err()
}

// TotalSize returns the sum of size_bytes across all recordings.
func TotalSize(db *DB) (int64, error) {
	var total sql.NullInt64
	err := db.QueryRow(`SELECT SUM(size_bytes) FROM recordings`).Scan(&total)
	return total.Int64, err
}

// StatsRecordings returns the total count and total size in bytes of all recordings.
func StatsRecordings(db *DB) (count int64, totalBytes int64, err error) {
	var c sql.NullInt64
	var b sql.NullInt64
	err = db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(size_bytes),0) FROM recordings`).Scan(&c, &b)
	return c.Int64, b.Int64, err
}

// UpdateRecordingEndedAt sets ended_at for a recording row where it is currently NULL.
func UpdateRecordingEndedAt(database *DB, path string, endedAt time.Time) error {
	_, err := database.Exec(
		`UPDATE recordings SET ended_at=? WHERE path=? AND ended_at IS NULL`,
		endedAt.UTC().Format(time.RFC3339), path,
	)
	return err
}

// LastRecordingPerCamera returns the most recent started_at per camera_id.
func LastRecordingPerCamera(db *DB) (map[string]time.Time, error) {
	rows, err := db.Query(`SELECT camera_id, MAX(started_at) FROM recordings GROUP BY camera_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string]time.Time)
	for rows.Next() {
		var cam string
		var ts string
		if err := rows.Scan(&cam, &ts); err != nil {
			return nil, err
		}
		t, err := time.Parse(time.RFC3339, ts)
		if err != nil {
			continue
		}
		result[cam] = t
	}
	return result, rows.Err()
}

// MarkAnalysisSkipped sets analysis_skipped=1 for the recording with the given id.
func MarkAnalysisSkipped(database *DB, id int64) error {
	_, err := database.Exec(`UPDATE recordings SET analysis_skipped=1 WHERE id=?`, id)
	return err
}

func nullTime(t time.Time) sql.NullString {
	if t.IsZero() {
		return sql.NullString{}
	}
	return sql.NullString{String: t.UTC().Format(time.RFC3339), Valid: true}
}
