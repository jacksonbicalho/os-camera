package db

import (
	"strings"
	"time"
)

type VideoAnalysisConfig struct {
	Enabled             bool    `json:"enabled"`
	ServiceURL          string  `json:"service_url"`
	Model               string  `json:"model"`
	ConfidenceThreshold float64 `json:"confidence_threshold"`
}

type Detection struct {
	ID         int64     `json:"id"`
	Label      string    `json:"label"`
	Confidence float64   `json:"confidence"`
	FrameCount int       `json:"frame_count"`
	CreatedAt  time.Time `json:"created_at"`
}

func GetVideoAnalysisConfig(d *DB) (VideoAnalysisConfig, error) {
	var cfg VideoAnalysisConfig
	var enabled int
	err := d.QueryRow(`
		SELECT enabled, service_url, model, confidence_threshold
		FROM video_analysis_config WHERE id=1`).
		Scan(&enabled, &cfg.ServiceURL, &cfg.Model, &cfg.ConfidenceThreshold)
	if err != nil {
		return VideoAnalysisConfig{}, err
	}
	cfg.Enabled = enabled != 0
	return cfg, nil
}

func UpdateVideoAnalysisConfig(d *DB, cfg VideoAnalysisConfig) error {
	enabled := 0
	if cfg.Enabled {
		enabled = 1
	}
	_, err := d.Exec(`
		UPDATE video_analysis_config
		SET enabled=?, service_url=?, model=?, confidence_threshold=?
		WHERE id=1`,
		enabled, cfg.ServiceURL, cfg.Model, cfg.ConfidenceThreshold)
	return err
}

func GetCameraAnalysisEnabled(d *DB, cameraID string) (bool, error) {
	var enabled int
	err := d.QueryRow(`SELECT enabled FROM camera_analysis_config WHERE camera_id=?`, cameraID).Scan(&enabled)
	if err != nil {
		// no row means default: enabled
		return true, nil
	}
	return enabled != 0, nil
}

func SetCameraAnalysisEnabled(d *DB, cameraID string, enabled bool) error {
	v := 0
	if enabled {
		v = 1
	}
	_, err := d.Exec(`
		INSERT INTO camera_analysis_config (camera_id, enabled) VALUES (?, ?)
		ON CONFLICT(camera_id) DO UPDATE SET enabled=excluded.enabled`,
		cameraID, v)
	return err
}

func InsertDetections(d *DB, path string, detections []Detection) error {
	var recordingID int64
	err := d.QueryRow(`SELECT id FROM recordings WHERE path=?`, path).Scan(&recordingID)
	if err != nil {
		return err
	}
	for _, det := range detections {
		_, err := d.Exec(`
			INSERT INTO detections (recording_id, label, confidence, frame_count)
			VALUES (?, ?, ?, ?)`,
			recordingID, det.Label, det.Confidence, det.FrameCount)
		if err != nil {
			return err
		}
	}
	return nil
}

func DetectionsByPaths(d *DB, paths []string) (map[string][]Detection, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(paths))
	args := make([]any, len(paths))
	for i, p := range paths {
		placeholders[i] = "?"
		args[i] = p
	}
	rows, err := d.Query(`
		SELECT rec.path, det.label, det.confidence, det.frame_count
		FROM detections det
		JOIN recordings rec ON rec.id = det.recording_id
		WHERE rec.path IN (`+strings.Join(placeholders, ",")+`)
		AND det.label != ''
		ORDER BY det.confidence DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[string][]Detection{}
	seen := map[string]map[string]bool{}
	for rows.Next() {
		var path string
		var det Detection
		if err := rows.Scan(&path, &det.Label, &det.Confidence, &det.FrameCount); err != nil {
			return nil, err
		}
		if seen[path] == nil {
			seen[path] = map[string]bool{}
		}
		if !seen[path][det.Label] {
			seen[path][det.Label] = true
			result[path] = append(result[path], det)
		}
	}
	return result, rows.Err()
}

func DetectionLabelsByPaths(d *DB, paths []string) (map[string][]string, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(paths))
	args := make([]any, len(paths))
	for i, p := range paths {
		placeholders[i] = "?"
		args[i] = p
	}
	rows, err := d.Query(`
		SELECT rec.path, det.label
		FROM detections det
		JOIN recordings rec ON rec.id = det.recording_id
		WHERE rec.path IN (`+strings.Join(placeholders, ",")+`)
		AND det.label != ''
		ORDER BY det.confidence DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[string][]string{}
	seen := map[string]map[string]bool{}
	for rows.Next() {
		var path, label string
		if err := rows.Scan(&path, &label); err != nil {
			return nil, err
		}
		if seen[path] == nil {
			seen[path] = map[string]bool{}
		}
		if !seen[path][label] {
			seen[path][label] = true
			result[path] = append(result[path], label)
		}
	}
	return result, rows.Err()
}

func ListDetectionsByPath(d *DB, path string) ([]Detection, error) {
	rows, err := d.Query(`
		SELECT det.id, det.label, det.confidence, det.frame_count, det.created_at
		FROM detections det
		JOIN recordings rec ON rec.id = det.recording_id
		WHERE rec.path = ?
		ORDER BY det.confidence DESC`, path)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []Detection
	for rows.Next() {
		var det Detection
		var createdAt string
		if err := rows.Scan(&det.ID, &det.Label, &det.Confidence, &det.FrameCount, &createdAt); err != nil {
			return nil, err
		}
		det.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		result = append(result, det)
	}
	return result, rows.Err()
}
