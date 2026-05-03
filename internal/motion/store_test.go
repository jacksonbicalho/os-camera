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

	st := newStore(dir)
	if err := st.record(cameraID, ts, 0.42); err != nil {
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
}

func TestStoreAppendsMultipleEvents(t *testing.T) {
	dir := t.TempDir()
	cameraID := "quintal"
	ts1 := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)
	ts2 := time.Date(2026, 5, 3, 10, 0, 5, 0, time.UTC)

	st := newStore(dir)
	st.record(cameraID, ts1, 0.1)
	st.record(cameraID, ts2, 0.2)

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
