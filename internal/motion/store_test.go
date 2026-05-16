package motion

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStoreWritesNDJSONEvent(t *testing.T) {
	dir := t.TempDir()
	cameraID := "entrada"
	ts := time.Date(2026, 5, 3, 14, 30, 0, 0, time.UTC)

	st := newStore(dir, nil)
	if err := st.record(cameraID, ts, 0.42, "20260503143000_motion.jpg", "", "", BBox{X: 0.1, Y: 0.2, W: 0.3, H: 0.4}); err != nil {
		t.Fatalf("record error: %v", err)
	}

	path := filepath.Join(dir, cameraID, "2026", "05", "03", "motion.ndjson")
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}
	defer f.Close()

	var event struct {
		Time  string  `json:"time"`
		Score float64 `json:"score"`
		Frame string  `json:"frame"`
		BBox  struct {
			X float64 `json:"x"`
			Y float64 `json:"y"`
			W float64 `json:"w"`
			H float64 `json:"h"`
		} `json:"bbox"`
	}
	if err := json.NewDecoder(bufio.NewReader(f)).Decode(&event); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if event.Time != "2026-05-03T14:30:00Z" {
		t.Errorf("unexpected time: %s", event.Time)
	}
	if event.Score < 0.41 || event.Score > 0.43 {
		t.Errorf("unexpected score: %f", event.Score)
	}
	if event.Frame != "20260503143000_motion.jpg" {
		t.Errorf("unexpected frame: %s", event.Frame)
	}
	if event.BBox.X != 0.1 || event.BBox.Y != 0.2 || event.BBox.W != 0.3 || event.BBox.H != 0.4 {
		t.Errorf("unexpected bbox: %+v", event.BBox)
	}
}

func TestStoreAppendsMultipleEvents(t *testing.T) {
	dir := t.TempDir()
	cameraID := "quintal"
	ts1 := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)
	ts2 := time.Date(2026, 5, 3, 10, 0, 5, 0, time.UTC)

	st := newStore(dir, nil)
	st.record(cameraID, ts1, 0.1, "20260503100000_motion.jpg", "", "", BBox{})
	st.record(cameraID, ts2, 0.2, "20260503100005_motion.jpg", "", "", BBox{})

	path := filepath.Join(dir, cameraID, "2026", "05", "03", "motion.ndjson")
	f, _ := os.Open(path)
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lines := 0
	for scanner.Scan() {
		lines++
	}
	if lines != 2 {
		t.Errorf("expected 2 lines, got %d", lines)
	}
}

func TestStoreWritesColorToNDJSON(t *testing.T) {
	dir := t.TempDir()
	ts := time.Date(2026, 5, 3, 14, 30, 0, 0, time.UTC)
	st := newStore(dir, nil)
	if err := st.record("cam1", ts, 0.1, "frame.jpg", "jardim", "#3b82f6", BBox{}); err != nil {
		t.Fatalf("record error: %v", err)
	}

	path := filepath.Join(dir, "cam1", "2026", "05", "03", "motion.ndjson")
	f, _ := os.Open(path)
	defer f.Close()
	var event map[string]any
	json.NewDecoder(f).Decode(&event)
	if event["label"] != "jardim" {
		t.Errorf("label: got %v, want jardim", event["label"])
	}
	if event["color"] != "#3b82f6" {
		t.Errorf("color: got %v, want #3b82f6", event["color"])
	}
}

// Evento sem frame (legado) deve ser lido corretamente com frame vazio
func TestStoreEmptyFrameName(t *testing.T) {
	dir := t.TempDir()
	ts := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)
	st := newStore(dir, nil)
	if err := st.record("cam1", ts, 0.05, "", "", "", BBox{}); err != nil {
		t.Fatalf("record error: %v", err)
	}

	path := filepath.Join(dir, "cam1", "2026", "05", "03", "motion.ndjson")
	f, _ := os.Open(path)
	defer f.Close()
	var event map[string]any
	json.NewDecoder(f).Decode(&event)
	if _, ok := event["frame"]; ok {
		val, _ := event["frame"].(string)
		if val != "" {
			t.Errorf("expected no frame field or empty, got %q", val)
		}
	}
}
