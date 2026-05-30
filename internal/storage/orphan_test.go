package storage_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"camera/internal/db"
	"camera/internal/storage"
)

func insertOrphan(t *testing.T, database *db.DB, camID, path string) {
	t.Helper()
	err := db.InsertRecording(database, db.Recording{
		CameraID:  camID,
		StartedAt: time.Now().UTC(),
		// EndedAt zero → NULL in DB
		Path:      path,
		SizeBytes: 48,
	})
	if err != nil {
		t.Fatalf("insertOrphan: %v", err)
	}
}

func TestCleanOrphanedRecordingsNoOrphans(t *testing.T) {
	database := openTestDB(t)
	n := storage.CleanOrphanedRecordings(database, discardLogger())
	if n != 0 {
		t.Fatalf("expected 0 removed, got %d", n)
	}
}

func TestCleanOrphanedRecordingsDeletesFileAndRow(t *testing.T) {
	database := openTestDB(t)
	createTestCamera(t, database, "cam1")
	dir := t.TempDir()
	path := filepath.Join(dir, "20260530130436.mp4")

	if err := os.WriteFile(path, []byte("fake mp4 data"), 0644); err != nil {
		t.Fatal(err)
	}
	insertOrphan(t, database, "cam1", path)

	n := storage.CleanOrphanedRecordings(database, discardLogger())

	if n != 1 {
		t.Fatalf("expected 1 removed, got %d", n)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("expected file to be deleted, stat error: %v", err)
	}

	orphans, err := db.ListOrphanedRecordings(database)
	if err != nil {
		t.Fatal(err)
	}
	if len(orphans) != 0 {
		t.Errorf("expected 0 orphans in DB, got %d", len(orphans))
	}
}

func TestCleanOrphanedRecordingsFileAlreadyGone(t *testing.T) {
	database := openTestDB(t)
	createTestCamera(t, database, "cam1")
	insertOrphan(t, database, "cam1", "/nonexistent/path/recording.mp4")

	// File does not exist on disk — should still remove the DB row.
	n := storage.CleanOrphanedRecordings(database, discardLogger())

	if n != 1 {
		t.Fatalf("expected 1 removed, got %d", n)
	}
	orphans, _ := db.ListOrphanedRecordings(database)
	if len(orphans) != 0 {
		t.Errorf("expected 0 orphans in DB, got %d", len(orphans))
	}
}

func TestCleanOrphanedRecordingsMultiple(t *testing.T) {
	database := openTestDB(t)
	createTestCamera(t, database, "cam1")
	dir := t.TempDir()

	paths := []string{
		filepath.Join(dir, "20260530130400.mp4"),
		filepath.Join(dir, "20260530130430.mp4"),
		filepath.Join(dir, "20260530130500.mp4"),
	}
	for _, p := range paths {
		if err := os.WriteFile(p, []byte("data"), 0644); err != nil {
			t.Fatal(err)
		}
		insertOrphan(t, database, "cam1", p)
	}

	n := storage.CleanOrphanedRecordings(database, discardLogger())

	if n != 3 {
		t.Fatalf("expected 3 removed, got %d", n)
	}
	for _, p := range paths {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("expected %s to be deleted", p)
		}
	}
}

func TestCleanOrphanedRecordingsPreservesFinalized(t *testing.T) {
	database := openTestDB(t)
	createTestCamera(t, database, "cam1")
	dir := t.TempDir()

	// Finalized recording (ended_at set) — must NOT be touched.
	finalizedPath := filepath.Join(dir, "finalized.mp4")
	if err := os.WriteFile(finalizedPath, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	err := db.InsertRecording(database, db.Recording{
		CameraID:  "cam1",
		StartedAt: time.Now().Add(-time.Minute).UTC(),
		EndedAt:   time.Now().UTC(),
		Path:      finalizedPath,
		SizeBytes: 1024,
	})
	if err != nil {
		t.Fatal(err)
	}

	n := storage.CleanOrphanedRecordings(database, discardLogger())

	if n != 0 {
		t.Fatalf("expected 0 removed, got %d", n)
	}
	if _, err := os.Stat(finalizedPath); err != nil {
		t.Errorf("finalized file should not be deleted: %v", err)
	}
}
