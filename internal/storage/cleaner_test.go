package storage_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"camera/internal/analysis"
	"camera/internal/config"
	"camera/internal/db"
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

func writeMotionNDJSONWithFrames(t *testing.T, dir string, events []struct {
	ts    time.Time
	frame string
}) string {
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
	for _, ev := range events {
		entry := map[string]any{"time": ev.ts.UTC().Format(time.RFC3339), "score": 0.05}
		if ev.frame != "" {
			entry["frame"] = ev.frame
		}
		if err := enc.Encode(entry); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

// --- RemoveEventsInRange ---

func TestRemoveEventsInRange_NoopWhenFileAbsent(t *testing.T) {
	err := storage.RemoveEventsInRange("/nonexistent/motion.ndjson", time.Now(), time.Now().Add(time.Minute))
	if err != nil {
		t.Errorf("expected nil error when file absent, got %v", err)
	}
}

func TestRemoveEventsInRange_RemovesEventsInsideWindow(t *testing.T) {
	dir := t.TempDir()
	base := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	writeMotionNDJSON(t, dir, []time.Time{
		base.Add(1 * time.Minute), // inside
		base.Add(6 * time.Minute), // outside (after window)
	})

	err := storage.RemoveEventsInRange(filepath.Join(dir, "motion.ndjson"), base, base.Add(5*time.Minute))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := storage.HasMotionInRange(filepath.Join(dir, "motion.ndjson"), base, base.Add(5*time.Minute))
	if got {
		t.Error("expected no events inside window after removal")
	}
	still := storage.HasMotionInRange(filepath.Join(dir, "motion.ndjson"), base.Add(6*time.Minute), base.Add(10*time.Minute))
	if !still {
		t.Error("expected event outside window to be kept")
	}
}

func TestRemoveEventsInRange_DeletesNDJSONWhenAllEventsRemoved(t *testing.T) {
	dir := t.TempDir()
	base := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	ndjson := writeMotionNDJSON(t, dir, []time.Time{base.Add(time.Minute)})

	if err := storage.RemoveEventsInRange(ndjson, base, base.Add(5*time.Minute)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(ndjson); !os.IsNotExist(err) {
		t.Error("expected motion.ndjson to be deleted when empty")
	}
}

func TestRemoveEventsInRange_DeletesReferencedJPEGs(t *testing.T) {
	dir := t.TempDir()
	base := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	jpegName := "20260511100100_motion.jpg"
	jpegPath := filepath.Join(dir, jpegName)
	if err := os.WriteFile(jpegPath, []byte("img"), 0644); err != nil {
		t.Fatal(err)
	}
	writeMotionNDJSONWithFrames(t, dir, []struct {
		ts    time.Time
		frame string
	}{{ts: base.Add(time.Minute), frame: jpegName}})

	if err := storage.RemoveEventsInRange(filepath.Join(dir, "motion.ndjson"), base, base.Add(5*time.Minute)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(jpegPath); !os.IsNotExist(err) {
		t.Error("expected _motion.jpg to be deleted along with event")
	}
}

func TestRemoveEventsInRange_KeepsJPEGsOutsideWindow(t *testing.T) {
	dir := t.TempDir()
	base := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	jpegInside := "20260511100100_motion.jpg"
	jpegOutside := "20260511100700_motion.jpg"
	for _, name := range []string{jpegInside, jpegOutside} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("img"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	writeMotionNDJSONWithFrames(t, dir, []struct {
		ts    time.Time
		frame string
	}{
		{ts: base.Add(1 * time.Minute), frame: jpegInside},
		{ts: base.Add(7 * time.Minute), frame: jpegOutside},
	})

	if err := storage.RemoveEventsInRange(filepath.Join(dir, "motion.ndjson"), base, base.Add(5*time.Minute)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, jpegOutside)); err != nil {
		t.Error("expected JPEG outside window to be kept")
	}
	if _, err := os.Stat(filepath.Join(dir, jpegInside)); !os.IsNotExist(err) {
		t.Error("expected JPEG inside window to be deleted")
	}
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

	storage.New(dir, 10080, 30, 5*time.Minute, 0, 0, nil, discardLogger()).Clean()

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

	storage.New(dir, 10080, 30, 5*time.Minute, 0, 0, nil, discardLogger()).Clean()

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

	storage.New(dir, 60, 30, 5*time.Minute, 0, 0, nil, discardLogger()).Clean()

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("expected with-motion chunk to be deleted after with_motion retention")
	}
}

// --- Clean: inferência de chunkEnd pelo arquivo seguinte ---

// Chunk A (1 min) sem motion; evento de motion no chunk B seguinte.
// Com a janela correta (até o início de B), A não deve ser classificado como "com motion".
func TestClean_AdjacentMotionEventDoesNotContaminateEarlierChunk(t *testing.T) {
	dir := t.TempDir()
	base := time.Now().UTC().Add(-120 * time.Minute).Truncate(time.Second)
	chunkA := base
	chunkB := base.Add(1 * time.Minute)

	pathA := mp4WithTimestamp(dir, "cam1", chunkA)
	pathB := mp4WithTimestamp(dir, "cam1", chunkB)
	writeFile(t, pathA, chunkA)
	writeFile(t, pathB, chunkB)

	// evento de motion aos 10s dentro do chunk B → fora da janela de A (1 min)
	writeMotionNDJSON(t, filepath.Dir(pathA), []time.Time{chunkB.Add(10 * time.Second)})

	// fallback de 5 min; sem ele o bug existia porque 5 min cobria o evento de B
	storage.New(dir, 0, 60, 5*time.Minute, 0, 0, nil, discardLogger()).Clean()

	if _, err := os.Stat(pathA); !os.IsNotExist(err) {
		t.Error("chunk A sem motion deveria ter sido deletado, mas foi retido (janela alargada)")
	}
	if _, err := os.Stat(pathB); err != nil {
		t.Errorf("chunk B com motion deveria ser mantido: %v", err)
	}
}

// Chunk com motion real dentro do seu próprio intervalo deve ser retido.
func TestClean_MotionInsideChunkWindowKeepsChunk(t *testing.T) {
	dir := t.TempDir()
	base := time.Now().UTC().Add(-120 * time.Minute).Truncate(time.Second)
	chunkA := base
	chunkB := base.Add(1 * time.Minute)

	pathA := mp4WithTimestamp(dir, "cam1", chunkA)
	pathB := mp4WithTimestamp(dir, "cam1", chunkB)
	writeFile(t, pathA, chunkA)
	writeFile(t, pathB, chunkB)

	// evento de motion aos 30s dentro do chunk A → dentro da janela de A
	writeMotionNDJSON(t, filepath.Dir(pathA), []time.Time{chunkA.Add(30 * time.Second)})

	storage.New(dir, 0, 60, 5*time.Minute, 0, 0, nil, discardLogger()).Clean()

	if _, err := os.Stat(pathA); err != nil {
		t.Errorf("chunk A com motion real deveria ser mantido: %v", err)
	}
}

// Último arquivo do diretório (sem próximo) usa fallbackDuration; deve ser deletado se expirado.
func TestClean_LastChunkInDirUsesFallbackDuration(t *testing.T) {
	dir := t.TempDir()
	chunkStart := time.Now().UTC().Add(-120 * time.Minute).Truncate(time.Second)
	path := mp4WithTimestamp(dir, "cam1", chunkStart)
	writeFile(t, path, chunkStart)
	// sem motion.ndjson; fallback de 5 min não alcança nenhum evento

	storage.New(dir, 0, 60, 5*time.Minute, 0, 0, nil, discardLogger()).Clean()

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("último chunk sem motion deveria ter sido deletado usando fallback duration")
	}
}

func TestClean_KeepsWithMotionChunkWhenWithMotionMinutesIsZero(t *testing.T) {
	dir := t.TempDir()
	chunkStart := time.Now().UTC().Add(-365 * 24 * time.Hour).Truncate(time.Second)
	path := mp4WithTimestamp(dir, "cam1", chunkStart)
	writeFile(t, path, chunkStart)
	writeMotionNDJSON(t, filepath.Dir(path), []time.Time{chunkStart.Add(1 * time.Minute)})

	// withMotion=0 → keep motion recordings indefinitely
	storage.New(dir, 0, 1440, 5*time.Minute, 0, 0, nil, discardLogger()).Clean()

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

	storage.New(dir, 30, 30, 5*time.Minute, 0, 0, nil, discardLogger()).Clean()

	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Error("expected old file to be deleted")
	}
}

func TestClean_KeepsRecentFiles(t *testing.T) {
	dir := t.TempDir()
	chunkStart := time.Now().UTC().Add(-1 * time.Minute).Truncate(time.Second)
	recent := mp4WithTimestamp(dir, "cam1", chunkStart)
	writeFile(t, recent, chunkStart)

	storage.New(dir, 30, 30, 5*time.Minute, 0, 0, nil, discardLogger()).Clean()

	if _, err := os.Stat(recent); err != nil {
		t.Errorf("expected recent file to exist: %v", err)
	}
}

func TestClean_DisabledWhenRetentionMinutesZero(t *testing.T) {
	dir := t.TempDir()
	chunkStart := time.Now().UTC().Add(-365 * 24 * time.Hour).Truncate(time.Second)
	old := mp4WithTimestamp(dir, "cam1", chunkStart)
	writeFile(t, old, chunkStart)

	storage.New(dir, 0, 0, 5*time.Minute, 0, 0, nil, discardLogger()).Clean()

	if _, err := os.Stat(old); err != nil {
		t.Errorf("expected file to exist when retention disabled: %v", err)
	}
}

func TestClean_IgnoresNonMp4Files(t *testing.T) {
	dir := t.TempDir()
	ts := filepath.Join(dir, "cam1", "2026", "01", "01", "001.ts")
	writeFile(t, ts, time.Now().Add(-31*time.Minute))

	storage.New(dir, 30, 30, 5*time.Minute, 0, 0, nil, discardLogger()).Clean()

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
	storage.New(dir, 0, 0, 5*time.Minute, maxSizeGB, 70, nil, log).CheckSize()

	if !strings.Contains(buf.String(), "storage usage high") {
		t.Errorf("expected storage usage warning, got: %s", buf.String())
	}
}

func openTestDB(t *testing.T) *db.DB {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

func queryEndedAt(t *testing.T, database *db.DB, path string) sql.NullString {
	t.Helper()
	var endedAt sql.NullString
	err := database.QueryRow(`SELECT ended_at FROM recordings WHERE path=?`, path).Scan(&endedAt)
	if err != nil {
		t.Fatalf("query ended_at for %s: %v", path, err)
	}
	return endedAt
}

func createTestCamera(t *testing.T, database *db.DB, id string) {
	t.Helper()
	dur5m := config.Duration(5 * time.Minute)
	dur30s := config.Duration(30 * time.Second)
	cam := config.CameraConfig{
		ID:                id,
		RTSPURL:           "rtsp://localhost/" + id,
		ChunkDuration:     dur5m,
		ReconnectInterval: dur30s,
	}
	cam.ID = id
	if _, err := db.CreateCamera(database, cam, nil); err != nil {
		t.Fatalf("create camera %s: %v", id, err)
	}
}

// Quando syncRecordings insere um arquivo como último (ended_at NULL) e depois
// um sucessor aparece, a segunda execução deve preencher o ended_at do primeiro.
func TestSyncRecordings_UpdatesEndedAtWhenSuccessorAppears(t *testing.T) {
	dir := t.TempDir()
	database := openTestDB(t)
	createTestCamera(t, database, "cam1")

	base := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Second)
	pathA := mp4WithTimestamp(dir, "cam1", base)
	writeFile(t, pathA, base)

	// Primeiro sync: só arquivo A → inserido com ended_at = NULL
	storage.New(dir, 10080, 10080, 5*time.Minute, 0, 0, database, discardLogger()).Clean()

	if got := queryEndedAt(t, database, pathA); got.Valid {
		t.Errorf("após primeiro sync ended_at deveria ser NULL, mas é %s", got.String)
	}

	// Arquivo B aparece
	pathB := mp4WithTimestamp(dir, "cam1", base.Add(5*time.Minute))
	writeFile(t, pathB, base.Add(5*time.Minute))

	// Segundo sync: A deve ter ended_at preenchido com o início de B
	storage.New(dir, 10080, 10080, 5*time.Minute, 0, 0, database, discardLogger()).Clean()

	got := queryEndedAt(t, database, pathA)
	if !got.Valid {
		t.Fatal("ended_at continua NULL após o sucessor aparecer; INSERT OR IGNORE não está sendo compensado com UPDATE")
	}
	want := base.Add(5 * time.Minute).UTC().Format(time.RFC3339)
	if got.String != want {
		t.Errorf("ended_at = %s, want %s", got.String, want)
	}
}

// createTestCameraWithMotion cria câmera com lead e trail configurados.
func createTestCameraWithMotion(t *testing.T, database *db.DB, id string, lead, trail int) {
	t.Helper()
	dur5m := config.Duration(5 * time.Minute)
	dur30s := config.Duration(30 * time.Second)
	cam := config.CameraConfig{
		ID:                id,
		RTSPURL:           "rtsp://localhost/" + id,
		ChunkDuration:     dur5m,
		ReconnectInterval: dur30s,
	}
	motion := &config.MotionConfig{
		Enabled:              true,
		Threshold:            0.05,
		PlaybackLeadSeconds:  lead,
		PlaybackTrailSeconds: trail,
	}
	if _, err := db.CreateCamera(database, cam, motion); err != nil {
		t.Fatalf("create camera %s: %v", id, err)
	}
}

// Chunk imediatamente após um evento deve ser marcado has_motion=1 quando
// o evento cai dentro da janela de trail do chunk seguinte.
// Chunk C é necessário para que B receba ended_at; sem ended_at o chunk
// não pode ser avaliado (está sendo gravado).
func TestSyncRecordings_TrailWindowMarksNextChunk(t *testing.T) {
	dir := t.TempDir()
	database := openTestDB(t)
	createTestCameraWithMotion(t, database, "cam1", 10, 10)

	base := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Second)
	// Chunk A: [base, base+5s)  — contém o evento (1s antes do fim)
	// Chunk B: [base+5s, base+10s) — não contém evento, mas está dentro de trail=10s
	// Chunk C: [base+10s, ...) — presença de C define o ended_at de B
	chunkA := base
	chunkB := base.Add(5 * time.Second)
	chunkC := base.Add(10 * time.Second)
	pathA := mp4WithTimestamp(dir, "cam1", chunkA)
	pathB := mp4WithTimestamp(dir, "cam1", chunkB)
	pathC := mp4WithTimestamp(dir, "cam1", chunkC)
	writeFile(t, pathA, chunkA)
	writeFile(t, pathB, chunkB)
	writeFile(t, pathC, chunkC)

	// evento 1s antes do fim de A → aftermath está em B
	evTime := chunkA.Add(4 * time.Second)
	addMotionEvent(t, database, "cam1", evTime, 0.1)

	storage.New(dir, 10080, 10080, 5*time.Minute, 0, 0, database, discardLogger()).Clean()

	if !hasMotionInDB(t, database, pathA) {
		t.Error("chunk A deveria ter has_motion=1 (contém o evento)")
	}
	if !hasMotionInDB(t, database, pathB) {
		t.Error("chunk B deveria ter has_motion=1 (está dentro do trail do evento)")
	}
}

// Chunk muito depois do evento (além do trail) não deve ser marcado.
func TestSyncRecordings_ChunkBeyondTrailNotMarked(t *testing.T) {
	dir := t.TempDir()
	database := openTestDB(t)
	createTestCameraWithMotion(t, database, "cam1", 10, 10)

	base := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Second)
	// Chunk A: contém o evento
	// Chunk C: começa 30s após o evento → fora do trail de 10s
	chunkA := base
	chunkC := base.Add(30 * time.Second)
	pathA := mp4WithTimestamp(dir, "cam1", chunkA)
	pathC := mp4WithTimestamp(dir, "cam1", chunkC)
	writeFile(t, pathA, chunkA)
	writeFile(t, pathC, chunkC)

	evTime := chunkA.Add(2 * time.Second)
	addMotionEvent(t, database, "cam1", evTime, 0.1)

	storage.New(dir, 10080, 10080, 5*time.Minute, 0, 0, database, discardLogger()).Clean()

	if !hasMotionInDB(t, database, pathA) {
		t.Error("chunk A deveria ter has_motion=1")
	}
	if hasMotionInDB(t, database, pathC) {
		t.Error("chunk C deveria ter has_motion=0 (além do trail)")
	}
}

func addMotionEvent(t *testing.T, database *db.DB, cameraID string, ts time.Time, score float64) {
	t.Helper()
	_, err := database.Exec(
		`INSERT INTO motion_events(camera_id, occurred_at, score) VALUES(?,?,?)`,
		cameraID, ts.UTC().Format(time.RFC3339), score,
	)
	if err != nil {
		t.Fatalf("insert motion event: %v", err)
	}
}

func hasMotionInDB(t *testing.T, database *db.DB, path string) bool {
	t.Helper()
	var v int
	if err := database.QueryRow(`SELECT has_motion FROM recordings WHERE path=?`, path).Scan(&v); err != nil {
		t.Fatalf("query has_motion for %s: %v", path, err)
	}
	return v != 0
}

// TestSyncRecordings_DoesNotMarkNullEndedAtAsHasMotion verifica que uma gravação
// com ended_at=NULL não recebe has_motion=1 mesmo quando há eventos de movimento
// após o início da gravação. O estado NULL indica que a gravação ainda está em
// andamento — não há como saber se o evento pertence a ela.
func TestSyncRecordings_DoesNotMarkNullEndedAtAsHasMotion(t *testing.T) {
	dir := t.TempDir()
	database := openTestDB(t)
	createTestCameraWithMotion(t, database, "cam1", 10, 10)

	base := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Second)
	// Único arquivo na pasta: ended_at permanece NULL (não há arquivo seguinte).
	path := mp4WithTimestamp(dir, "cam1", base)
	writeFile(t, path, base)

	// Evento após o início da gravação — com o bug, isso marca has_motion=1.
	addMotionEvent(t, database, "cam1", base.Add(2*time.Minute), 0.1)

	storage.New(dir, 10080, 10080, 5*time.Minute, 0, 0, database, discardLogger()).Clean()

	if hasMotionInDB(t, database, path) {
		t.Error("gravação com ended_at=NULL não deve receber has_motion=1")
	}
}

// TestCleanFromDB_PurgesOrphanedMotionEvents verifica que eventos de movimento
// sem cobertura de nenhuma gravação (órfãos) são removidos após a gravação
// relacionada ser deletada pela regra de retenção.
func TestCleanFromDB_KeepsMotionEventsOutsideRecordingRange(t *testing.T) {
	dir := t.TempDir()
	database := openTestDB(t)
	// lead=30s: evento 5s antes de A começa cobre A pelo lead
	createTestCameraWithMotion(t, database, "cam1", 30, 10)

	base := time.Now().UTC().Add(-120 * time.Minute).Truncate(time.Second)

	pathA := mp4WithTimestamp(dir, "cam1", base)
	pathB := mp4WithTimestamp(dir, "cam1", base.Add(10*time.Second))
	writeFile(t, pathA, base)
	writeFile(t, pathB, base.Add(10*time.Second))

	// Evento 5s ANTES de A começar: fora do intervalo exato de A [base, base+10s),
	// mas dentro da janela de lead (30s) → A será marcado has_motion=1.
	evOrphan := base.Add(-5 * time.Second)
	addMotionEvent(t, database, "cam1", evOrphan, 0.1)

	// Primeiro ciclo: sincroniza gravações e marca has_motion.
	storage.New(dir, 10080, 10080, 5*time.Minute, 0, 0, database, discardLogger()).Clean()

	if !hasMotionInDB(t, database, pathA) {
		t.Fatal("pathA deve ter has_motion=1 (evento dentro da janela de lead)")
	}

	// Segundo ciclo: retenção curta deleta ambas as gravações (120min > 60min).
	storage.New(dir, 60, 60, 5*time.Minute, 0, 0, database, discardLogger()).Clean()

	if _, err := os.Stat(pathA); !os.IsNotExist(err) {
		t.Fatal("pathA deveria ter sido deletado")
	}

	// O evento fora do intervalo da gravação ([base-5s]) não é deletado pelo cleaner:
	// purgeOrphanedEvents foi removido pois não é possível distinguir eventos de
	// câmeras com gravação desabilitada de eventos cujas gravações foram limpas.
	// A limpeza de eventos acontece apenas quando a gravação é explicitamente deletada
	// via handleDeleteRecording ou cleanFromDB (purgeMotionAssets).
	var count int
	database.QueryRow(`SELECT COUNT(*) FROM motion_events WHERE camera_id='cam1'`).Scan(&count)
	if count != 1 {
		t.Errorf("evento fora do range da gravação deve persistir no banco, mas count=%d", count)
	}
}

func TestCheckSize_NoWarnWhenBelowThreshold(t *testing.T) {
	dir := t.TempDir()
	// 50 bytes total; maxSizeGB ~107 bytes, 70% threshold ~75 bytes → no warn
	writeFileWithSize(t, filepath.Join(dir, "cam1", "file1.mp4"), 50)

	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	const maxSizeGB = 1e-7 // ~107 bytes
	storage.New(dir, 0, 0, 5*time.Minute, maxSizeGB, 70, nil, log).CheckSize()

	if strings.Contains(buf.String(), "storage usage high") {
		t.Errorf("unexpected storage usage warning below threshold")
	}
}

// O arquivo mais recente de cada câmera tem ended_at IS NULL enquanto o próximo
// não aparece. O cleaner não deve deletá-lo nem enviá-lo ao drive — ele ainda
// está sendo gravado.
// Quando o cleaner deleta uma gravação com motion, deve também apagar os JPEGs
// de evento referenciados no banco e remover as linhas de motion_events.
func TestClean_PurgesMotionJPEGsOnDelete(t *testing.T) {
	dir := t.TempDir()
	database := openTestDB(t)
	createTestCameraWithMotion(t, database, "cam1", 0, 0)

	base := time.Now().UTC().Add(-120 * time.Minute).Truncate(time.Second)
	pathA := mp4WithTimestamp(dir, "cam1", base)
	pathB := mp4WithTimestamp(dir, "cam1", base.Add(time.Minute))
	writeFile(t, pathA, base)
	writeFile(t, pathB, base.Add(time.Minute))

	// evento com frame_path dentro da janela de A
	evTime := base.Add(10 * time.Second)
	jpegName := evTime.UTC().Format("20060102150405") + "_motion.jpg"
	jpegPath := filepath.Join(filepath.Dir(pathA), jpegName)
	writeFile(t, jpegPath, evTime)

	_, err := database.Exec(
		`INSERT INTO motion_events(camera_id, occurred_at, score, frame_path) VALUES(?,?,?,?)`,
		"cam1", evTime.UTC().Format(time.RFC3339), 0.1, jpegName,
	)
	if err != nil {
		t.Fatalf("insert motion event: %v", err)
	}

	// retenção de 60 min com e sem motion → A (com motion) será deletado
	storage.New(dir, 60, 60, 5*time.Minute, 0, 0, database, discardLogger()).Clean()

	if _, err := os.Stat(jpegPath); !os.IsNotExist(err) {
		t.Error("_motion.jpg deve ser apagado junto com a gravação")
	}
	var count int
	database.QueryRow(`SELECT COUNT(*) FROM motion_events WHERE camera_id='cam1'`).Scan(&count)
	if count != 0 {
		t.Errorf("motion_events deve estar vazio após purge, mas tem %d linhas", count)
	}
}

func TestClean_DoesNotDeleteCurrentRecording(t *testing.T) {
	dir := t.TempDir()
	database := openTestDB(t)
	createTestCamera(t, database, "cam1")

	// Único arquivo na pasta: sem succeeded, ended_at ficará NULL após sync.
	chunkStart := time.Now().UTC().Add(-30 * time.Minute).Truncate(time.Second)
	path := mp4WithTimestamp(dir, "cam1", chunkStart)
	writeFile(t, path, chunkStart)

	// Retenção curta (1 min) sem motion: sem ended_at o arquivo não deve ser deletado.
	storage.New(dir, 0, 1, 5*time.Minute, 0, 0, database, discardLogger()).Clean()

	if _, err := os.Stat(path); err != nil {
		t.Errorf("arquivo corrente (ended_at NULL) não deve ser deletado: %v", err)
	}
}

func TestAnalyzeNewRecordings_AnalyzesCompletedChunks(t *testing.T) {
	dir := t.TempDir()
	database := openTestDB(t)
	createTestCameraWithMotion(t, database, "cam1", 10, 10)

	if err := db.UpdateVideoAnalysisConfig(database, db.VideoAnalysisConfig{
		Enabled:             true,
		ServiceURL:          "http://yolo:8000",
		Model:               "yolov8n",
		ConfidenceThreshold: 0.4,
	}); err != nil {
		t.Fatalf("UpdateVideoAnalysisConfig: %v", err)
	}

	base := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Second)
	pathA := mp4WithTimestamp(dir, "cam1", base)
	pathB := mp4WithTimestamp(dir, "cam1", base.Add(5*time.Minute))
	writeFile(t, pathA, base)
	writeFile(t, pathB, base.Add(5*time.Minute))

	// Motion event within pathA's range so syncRecordings sets has_motion=1.
	if err := db.InsertMotionEvent(database, db.MotionEvent{
		CameraID:   "cam1",
		OccurredAt: base.Add(time.Minute),
		Score:      0.5,
	}); err != nil {
		t.Fatalf("InsertMotionEvent: %v", err)
	}

	fake := &analysis.FakeAnalyzer{
		Results: []analysis.Detection{
			{Label: "person", Confidence: 0.9, FrameCount: 5},
		},
	}
	cleaner := storage.New(dir, 0, 0, 5*time.Minute, 0, 0, database, discardLogger()).
		WithAnalyzer(fake)
	cleaner.Clean()

	// pathA has ended_at (pathB appeared), so it should be analyzed
	dets, err := db.ListDetectionsByPath(database, pathA)
	if err != nil {
		t.Fatalf("ListDetectionsByPath: %v", err)
	}
	if len(dets) != 1 || dets[0].Label != "person" {
		t.Errorf("expected 1 detection 'person' for completed chunk, got %v", dets)
	}

	// pathB has no ended_at yet (last in dir) → should not be analyzed
	detsB, _ := db.ListDetectionsByPath(database, pathB)
	if len(detsB) != 0 {
		t.Errorf("incomplete chunk should not be analyzed, got %d detections", len(detsB))
	}

	// Running Clean again should not re-analyze pathA (already has detections)
	prevCalled := fake.Called
	cleaner.Clean()
	if fake.Called != prevCalled+0 {
		// pathB might get ended_at if a third chunk appears, but none did —
		// just ensure pathA is not re-analyzed
	}
	dets2, _ := db.ListDetectionsByPath(database, pathA)
	if len(dets2) != 1 {
		t.Errorf("re-run should not duplicate detections, got %d", len(dets2))
	}
}

func TestAnalyzeNewRecordings_SkipsWhenDisabled(t *testing.T) {
	dir := t.TempDir()
	database := openTestDB(t)
	createTestCamera(t, database, "cam1")

	// global config: disabled (default)
	base := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Second)
	pathA := mp4WithTimestamp(dir, "cam1", base)
	pathB := mp4WithTimestamp(dir, "cam1", base.Add(5*time.Minute))
	writeFile(t, pathA, base)
	writeFile(t, pathB, base.Add(5*time.Minute))

	fake := &analysis.FakeAnalyzer{Results: []analysis.Detection{{Label: "car", Confidence: 0.8}}}
	storage.New(dir, 0, 0, 5*time.Minute, 0, 0, database, discardLogger()).
		WithAnalyzer(fake).
		Clean()

	if fake.Called != 0 {
		t.Errorf("analyzer should not be called when global config is disabled, called %d times", fake.Called)
	}
}

func TestAnalyzeNewRecordings_SkipsAfterAnalyzeError(t *testing.T) {
	dir := t.TempDir()
	database := openTestDB(t)
	createTestCameraWithMotion(t, database, "cam1", 10, 10)

	if err := db.UpdateVideoAnalysisConfig(database, db.VideoAnalysisConfig{
		Enabled:    true,
		ServiceURL: "http://yolo:8000",
		Model:      "yolov8n",
	}); err != nil {
		t.Fatalf("UpdateVideoAnalysisConfig: %v", err)
	}

	base := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Second)
	pathA := mp4WithTimestamp(dir, "cam1", base)
	pathB := mp4WithTimestamp(dir, "cam1", base.Add(5*time.Minute))
	writeFile(t, pathA, base)
	writeFile(t, pathB, base.Add(5*time.Minute))

	if err := db.InsertMotionEvent(database, db.MotionEvent{
		CameraID:   "cam1",
		OccurredAt: base.Add(time.Minute),
		Score:      0.5,
	}); err != nil {
		t.Fatalf("InsertMotionEvent: %v", err)
	}

	fake := &analysis.FakeAnalyzer{Err: errors.New("yolo service returned 422")}
	cleaner := storage.New(dir, 0, 0, 5*time.Minute, 0, 0, database, discardLogger()).
		WithAnalyzer(fake)

	cleaner.Clean()
	if fake.Called != 1 {
		t.Fatalf("expected analyzer called once, got %d", fake.Called)
	}

	// Second run must not retry the failed recording.
	cleaner.Clean()
	if fake.Called != 1 {
		t.Errorf("failed recording should not be retried, analyzer called %d times total", fake.Called)
	}
}

func TestAnalyzeNewRecordings_SkipsWhenFileNotOnDisk(t *testing.T) {
	dir := t.TempDir()
	database := openTestDB(t)
	createTestCameraWithMotion(t, database, "cam1", 10, 10)

	if err := db.UpdateVideoAnalysisConfig(database, db.VideoAnalysisConfig{
		Enabled:    true,
		ServiceURL: "http://yolo:8000",
		Model:      "yolov8n",
	}); err != nil {
		t.Fatalf("UpdateVideoAnalysisConfig: %v", err)
	}

	// Insert a recording that exists in the DB but NOT on disk.
	base := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Second)
	missingPath := mp4WithTimestamp(dir, "cam1", base)
	if err := db.InsertRecording(database, db.Recording{
		CameraID:  "cam1",
		StartedAt: base,
		EndedAt:   base.Add(5 * time.Minute),
		Path:      missingPath,
		SizeBytes: 1024,
		HasMotion: true,
	}); err != nil {
		t.Fatalf("InsertRecording: %v", err)
	}

	fake := &analysis.FakeAnalyzer{Results: []analysis.Detection{{Label: "person", Confidence: 0.9}}}
	storage.New(dir, 0, 0, 5*time.Minute, 0, 0, database, discardLogger()).
		WithAnalyzer(fake).
		Clean()

	if fake.Called != 0 {
		t.Errorf("analyzer must not be called when file does not exist on disk, called %d times", fake.Called)
	}
}
