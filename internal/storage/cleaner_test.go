package storage_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"camera/internal/storage"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func writeFile(t *testing.T, path string, mtime time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatal(err)
	}
}

func writeFileWithSize(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, bytes.Repeat([]byte{0}, size), 0644); err != nil {
		t.Fatal(err)
	}
}

func writeMotionNDJSON(t *testing.T, dir string, events []time.Time) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "motion.ndjson")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, ts := range events {
		if err := enc.Encode(map[string]any{"time": ts.UTC().Format(time.RFC3339), "score": 0.05}); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func mp4WithTimestamp(dir, cameraID string, ts time.Time) string {
	day := ts.UTC().Format("2006/01/02")
	name := ts.UTC().Format("20060102150405") + ".mp4"
	return filepath.Join(dir, cameraID, day, name)
}

// --- HasMotionInRange ---

func TestHasMotionInRange_WithEventInRange(t *testing.T) {
	dir := t.TempDir()
	ts := time.Now().UTC().Truncate(time.Second)
	start := ts.Add(-5 * time.Minute)
	end := ts.Add(5 * time.Minute)
	writeMotionNDJSON(t, dir, []time.Time{ts})

	got := storage.HasMotionInRange(filepath.Join(dir, "motion.ndjson"), start, end)

	if !got {
		t.Error("expected true: event is inside range")
	}
}

func TestHasMotionInRange_WithEventOutsideRange(t *testing.T) {
	dir := t.TempDir()
	ts := time.Now().UTC().Truncate(time.Second)
	start := ts.Add(5 * time.Minute)
	end := ts.Add(10 * time.Minute)
	writeMotionNDJSON(t, dir, []time.Time{ts})

	got := storage.HasMotionInRange(filepath.Join(dir, "motion.ndjson"), start, end)

	if got {
		t.Error("expected false: event is outside range")
	}
}

func TestHasMotionInRange_AtBoundaryStartIncluded(t *testing.T) {
	dir := t.TempDir()
	ts := time.Now().UTC().Truncate(time.Second)
	writeMotionNDJSON(t, dir, []time.Time{ts})

	got := storage.HasMotionInRange(filepath.Join(dir, "motion.ndjson"), ts, ts.Add(5*time.Minute))

	if !got {
		t.Error("expected true: event at start boundary is included [start, end)")
	}
}

func TestHasMotionInRange_AtBoundaryEndExcluded(t *testing.T) {
	dir := t.TempDir()
	ts := time.Now().UTC().Truncate(time.Second)
	writeMotionNDJSON(t, dir, []time.Time{ts})

	got := storage.HasMotionInRange(filepath.Join(dir, "motion.ndjson"), ts.Add(-5*time.Minute), ts)

	if got {
		t.Error("expected false: event at end boundary is excluded [start, end)")
	}
}

func TestHasMotionInRange_NoFileReturnsFalse(t *testing.T) {
	got := storage.HasMotionInRange("/nonexistent/motion.ndjson", time.Now(), time.Now().Add(time.Minute))
	if got {
		t.Error("expected false when ndjson file does not exist")
	}
}

func TestHasMotionInRange_EmptyFileReturnsFalse(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "motion.ndjson")
	if err := os.WriteFile(path, nil, 0644); err != nil {
		t.Fatal(err)
	}
	got := storage.HasMotionInRange(path, time.Now(), time.Now().Add(time.Minute))
	if got {
		t.Error("expected false for empty ndjson")
	}
}

// --- ChunkStartFromName ---

func TestChunkStartFromName_ValidName(t *testing.T) {
	got, err := storage.ChunkStartFromName("20260509120000.mp4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("expected %v, got %v", want, got)
	}
}

func TestChunkStartFromName_InvalidName(t *testing.T) {
	_, err := storage.ChunkStartFromName("recording.mp4")
	if err == nil {
		t.Error("expected error for non-timestamp filename")
	}
}

func TestChunkStartFromName_StripsExtension(t *testing.T) {
	got, err := storage.ChunkStartFromName("20260101000000.mp4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("expected %v, got %v", want, got)
	}
}

// --- Clean with differentiated retention ---

func TestClean_DeletesWithoutMotionChunkAfterWithoutMotionRetention(t *testing.T) {
	dir := t.TempDir()
	chunkStart := time.Now().UTC().Add(-31 * time.Minute).Truncate(time.Second)
	path := mp4WithTimestamp(dir, "cam1", chunkStart)
	writeFile(t, path, chunkStart)
	// no motion.ndjson → no motion

	storage.New(dir, 10080, 30, 5*time.Minute, 0, 0, discardLogger()).Clean()

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("expected without-motion chunk to be deleted after without_motion retention")
	}
}

func TestClean_KeepsWithMotionChunkAfterWithoutMotionRetention(t *testing.T) {
	dir := t.TempDir()
	chunkStart := time.Now().UTC().Add(-31 * time.Minute).Truncate(time.Second)
	path := mp4WithTimestamp(dir, "cam1", chunkStart)
	writeFile(t, path, chunkStart)
	// write motion event inside chunk range
	writeMotionNDJSON(t, filepath.Dir(path), []time.Time{chunkStart.Add(1 * time.Minute)})

	storage.New(dir, 10080, 30, 5*time.Minute, 0, 0, discardLogger()).Clean()

	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected with-motion chunk to be kept (within with_motion retention): %v", err)
	}
}

func TestClean_DeletesWithMotionChunkAfterWithMotionRetention(t *testing.T) {
	dir := t.TempDir()
	chunkStart := time.Now().UTC().Add(-61 * time.Minute).Truncate(time.Second)
	path := mp4WithTimestamp(dir, "cam1", chunkStart)
	writeFile(t, path, chunkStart)
	writeMotionNDJSON(t, filepath.Dir(path), []time.Time{chunkStart.Add(1 * time.Minute)})

	storage.New(dir, 60, 30, 5*time.Minute, 0, 0, discardLogger()).Clean()

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("expected with-motion chunk to be deleted after with_motion retention")
	}
}

func TestClean_KeepsWithMotionChunkWhenWithMotionMinutesIsZero(t *testing.T) {
	dir := t.TempDir()
	chunkStart := time.Now().UTC().Add(-365 * 24 * time.Hour).Truncate(time.Second)
	path := mp4WithTimestamp(dir, "cam1", chunkStart)
	writeFile(t, path, chunkStart)
	writeMotionNDJSON(t, filepath.Dir(path), []time.Time{chunkStart.Add(1 * time.Minute)})

	// withMotion=0 → keep motion recordings indefinitely
	storage.New(dir, 0, 1440, 5*time.Minute, 0, 0, discardLogger()).Clean()

	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected motion chunk to be kept when with_motion_minutes=0: %v", err)
	}
}

// --- Existing tests (updated for new signature) ---

func TestClean_DeletesOldFiles(t *testing.T) {
	dir := t.TempDir()
	chunkStart := time.Now().UTC().Add(-31 * time.Minute).Truncate(time.Second)
	old := mp4WithTimestamp(dir, "cam1", chunkStart)
	writeFile(t, old, chunkStart)

	storage.New(dir, 30, 30, 5*time.Minute, 0, 0, discardLogger()).Clean()

	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Error("expected old file to be deleted")
	}
}

func TestClean_KeepsRecentFiles(t *testing.T) {
	dir := t.TempDir()
	chunkStart := time.Now().UTC().Add(-1 * time.Minute).Truncate(time.Second)
	recent := mp4WithTimestamp(dir, "cam1", chunkStart)
	writeFile(t, recent, chunkStart)

	storage.New(dir, 30, 30, 5*time.Minute, 0, 0, discardLogger()).Clean()

	if _, err := os.Stat(recent); err != nil {
		t.Errorf("expected recent file to exist: %v", err)
	}
}

func TestClean_DisabledWhenRetentionMinutesZero(t *testing.T) {
	dir := t.TempDir()
	chunkStart := time.Now().UTC().Add(-365 * 24 * time.Hour).Truncate(time.Second)
	old := mp4WithTimestamp(dir, "cam1", chunkStart)
	writeFile(t, old, chunkStart)

	storage.New(dir, 0, 0, 5*time.Minute, 0, 0, discardLogger()).Clean()

	if _, err := os.Stat(old); err != nil {
		t.Errorf("expected file to exist when retention disabled: %v", err)
	}
}

func TestClean_IgnoresNonMp4Files(t *testing.T) {
	dir := t.TempDir()
	ts := filepath.Join(dir, "cam1", "2026", "01", "01", "001.ts")
	writeFile(t, ts, time.Now().Add(-31*time.Minute))

	storage.New(dir, 30, 30, 5*time.Minute, 0, 0, discardLogger()).Clean()

	if _, err := os.Stat(ts); err != nil {
		t.Errorf("expected non-mp4 file to be preserved: %v", err)
	}
}

func TestCheckSize_LogsWarnWhenAboveThreshold(t *testing.T) {
	dir := t.TempDir()
	// 200 bytes total; maxSizeGB ~107 bytes, 70% threshold ~75 bytes → should warn
	writeFileWithSize(t, filepath.Join(dir, "cam1", "file1.mp4"), 100)
	writeFileWithSize(t, filepath.Join(dir, "cam1", "file2.mp4"), 100)

	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	const maxSizeGB = 1e-7 // ~107 bytes
	storage.New(dir, 0, 0, 5*time.Minute, maxSizeGB, 70, log).CheckSize()

	if !strings.Contains(buf.String(), "storage usage high") {
		t.Errorf("expected storage usage warning, got: %s", buf.String())
	}
}

func TestCheckSize_NoWarnWhenBelowThreshold(t *testing.T) {
	dir := t.TempDir()
	// 50 bytes total; maxSizeGB ~107 bytes, 70% threshold ~75 bytes → no warn
	writeFileWithSize(t, filepath.Join(dir, "cam1", "file1.mp4"), 50)

	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	const maxSizeGB = 1e-7 // ~107 bytes
	storage.New(dir, 0, 0, 5*time.Minute, maxSizeGB, 70, log).CheckSize()

	if strings.Contains(buf.String(), "storage usage high") {
		t.Errorf("unexpected storage usage warning below threshold")
	}
}
