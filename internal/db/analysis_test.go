package db_test

import (
	"testing"
	"time"

	"camera/internal/db"
)

func TestVideoAnalysisConfig_DefaultAndUpdate(t *testing.T) {
	database := openTestDB(t)

	cfg, err := db.GetVideoAnalysisConfig(database)
	if err != nil {
		t.Fatalf("GetVideoAnalysisConfig: %v", err)
	}
	if cfg.Enabled {
		t.Error("default enabled should be false")
	}
	if cfg.Model != "yolov8n" {
		t.Errorf("default model = %q, want yolov8n", cfg.Model)
	}
	if cfg.ConfidenceThreshold != 0.4 {
		t.Errorf("default confidence = %v, want 0.4", cfg.ConfidenceThreshold)
	}

	cfg.Enabled = true
	cfg.ServiceURL = "http://yolo:8000"
	cfg.Model = "yolov8s"
	cfg.ConfidenceThreshold = 0.5
	if err := db.UpdateVideoAnalysisConfig(database, cfg); err != nil {
		t.Fatalf("UpdateVideoAnalysisConfig: %v", err)
	}

	got, err := db.GetVideoAnalysisConfig(database)
	if err != nil {
		t.Fatalf("GetVideoAnalysisConfig after update: %v", err)
	}
	if !got.Enabled {
		t.Error("enabled should be true after update")
	}
	if got.ServiceURL != "http://yolo:8000" {
		t.Errorf("ServiceURL = %q, want http://yolo:8000", got.ServiceURL)
	}
	if got.Model != "yolov8s" {
		t.Errorf("Model = %q, want yolov8s", got.Model)
	}
	if got.ConfidenceThreshold != 0.5 {
		t.Errorf("ConfidenceThreshold = %v, want 0.5", got.ConfidenceThreshold)
	}
}

func TestCameraAnalysisConfig_DefaultAndSet(t *testing.T) {
	database := openTestDB(t)
	ensureCamera(t, database, "cam1")

	enabled, err := db.GetCameraAnalysisEnabled(database, "cam1")
	if err != nil {
		t.Fatalf("GetCameraAnalysisEnabled: %v", err)
	}
	if !enabled {
		t.Error("default per-camera enabled should be true")
	}

	if err := db.SetCameraAnalysisEnabled(database, "cam1", false); err != nil {
		t.Fatalf("SetCameraAnalysisEnabled: %v", err)
	}
	enabled, err = db.GetCameraAnalysisEnabled(database, "cam1")
	if err != nil {
		t.Fatalf("GetCameraAnalysisEnabled after set: %v", err)
	}
	if enabled {
		t.Error("expected enabled=false after set")
	}

	if err := db.SetCameraAnalysisEnabled(database, "cam1", true); err != nil {
		t.Fatalf("SetCameraAnalysisEnabled true: %v", err)
	}
	enabled, _ = db.GetCameraAnalysisEnabled(database, "cam1")
	if !enabled {
		t.Error("expected enabled=true after re-enable")
	}
}

func TestDetections_InsertAndList(t *testing.T) {
	database := openTestDB(t)
	ensureCamera(t, database, "cam1")

	if err := db.InsertRecording(database, db.Recording{
		CameraID:  "cam1",
		StartedAt: time.Now().Add(-5 * time.Minute),
		EndedAt:   time.Now(),
		Path:      "cam1/2024/01/01/120000.mp4",
		SizeBytes: 1024,
	}); err != nil {
		t.Fatalf("InsertRecording: %v", err)
	}

	detections := []db.Detection{
		{Label: "person", Confidence: 0.92, FrameCount: 10},
		{Label: "car", Confidence: 0.78, FrameCount: 3},
	}
	if err := db.InsertDetections(database, "cam1/2024/01/01/120000.mp4", detections); err != nil {
		t.Fatalf("InsertDetections: %v", err)
	}

	got, err := db.ListDetectionsByPath(database, "cam1/2024/01/01/120000.mp4")
	if err != nil {
		t.Fatalf("ListDetectionsByPath: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 detections, got %d", len(got))
	}

	labels := map[string]bool{}
	for _, d := range got {
		labels[d.Label] = true
	}
	if !labels["person"] || !labels["car"] {
		t.Errorf("expected person and car labels, got %v", labels)
	}
}

func TestDetectionLabelsByPaths_ReturnsBatchLabels(t *testing.T) {
	database := openTestDB(t)
	ensureCamera(t, database, "cam1")

	for _, path := range []string{"cam1/2024/01/01/120000.mp4", "cam1/2024/01/01/120500.mp4"} {
		if err := db.InsertRecording(database, db.Recording{
			CameraID:  "cam1",
			StartedAt: time.Now().Add(-5 * time.Minute),
			EndedAt:   time.Now(),
			Path:      path,
			SizeBytes: 512,
		}); err != nil {
			t.Fatalf("InsertRecording(%s): %v", path, err)
		}
	}
	if err := db.InsertDetections(database, "cam1/2024/01/01/120000.mp4", []db.Detection{
		{Label: "person", Confidence: 0.9, FrameCount: 3},
		{Label: "car", Confidence: 0.7, FrameCount: 1},
	}); err != nil {
		t.Fatalf("InsertDetections: %v", err)
	}
	if err := db.InsertDetections(database, "cam1/2024/01/01/120500.mp4", []db.Detection{
		{Label: "dog", Confidence: 0.8, FrameCount: 2},
	}); err != nil {
		t.Fatalf("InsertDetections: %v", err)
	}

	result, err := db.DetectionLabelsByPaths(database, []string{
		"cam1/2024/01/01/120000.mp4",
		"cam1/2024/01/01/120500.mp4",
		"cam1/2024/01/01/nonexistent.mp4",
	})
	if err != nil {
		t.Fatalf("DetectionLabelsByPaths: %v", err)
	}
	if len(result["cam1/2024/01/01/120000.mp4"]) != 2 {
		t.Errorf("expected 2 labels for first path, got %v", result["cam1/2024/01/01/120000.mp4"])
	}
	if len(result["cam1/2024/01/01/120500.mp4"]) != 1 || result["cam1/2024/01/01/120500.mp4"][0] != "dog" {
		t.Errorf("expected [dog] for second path, got %v", result["cam1/2024/01/01/120500.mp4"])
	}
	if result["cam1/2024/01/01/nonexistent.mp4"] != nil {
		t.Errorf("expected nil for nonexistent path, got %v", result["cam1/2024/01/01/nonexistent.mp4"])
	}
}

func TestDetections_DeleteCascade(t *testing.T) {
	database := openTestDB(t)
	ensureCamera(t, database, "cam1")

	if err := db.InsertRecording(database, db.Recording{
		CameraID:  "cam1",
		StartedAt: time.Now().Add(-5 * time.Minute),
		EndedAt:   time.Now(),
		Path:      "cam1/2024/01/01/120001.mp4",
		SizeBytes: 512,
	}); err != nil {
		t.Fatalf("InsertRecording: %v", err)
	}
	if err := db.InsertDetections(database, "cam1/2024/01/01/120001.mp4", []db.Detection{
		{Label: "dog", Confidence: 0.85, FrameCount: 2},
	}); err != nil {
		t.Fatalf("InsertDetections: %v", err)
	}

	if err := db.DeleteRecording(database, "cam1/2024/01/01/120001.mp4"); err != nil {
		t.Fatalf("DeleteRecording: %v", err)
	}

	got, err := db.ListDetectionsByPath(database, "cam1/2024/01/01/120001.mp4")
	if err != nil {
		t.Fatalf("ListDetectionsByPath after delete: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0 detections after cascade delete, got %d", len(got))
	}
}
