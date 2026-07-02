package storage

import (
	"log/slog"
	"os"
	"path/filepath"
)

// CleanOrphanedSegments removes HLS segment directories under segmentsPath whose
// name is not a known camera ID — e.g. cameras deleted while the server was down,
// or leftovers from before the per-camera transport gating existed. validCameraIDs
// is the set of current camera IDs; any other top-level directory is removed.
//
// Meant to run at startup. Returns the number of directories removed.
func CleanOrphanedSegments(segmentsPath string, validCameraIDs map[string]bool, log *slog.Logger) int {
	if segmentsPath == "" {
		return 0
	}
	entries, err := os.ReadDir(segmentsPath)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Warn("segment cleanup: read dir failed", "path", segmentsPath, "err", err)
		}
		return 0
	}
	removed := 0
	for _, e := range entries {
		if !e.IsDir() || validCameraIDs[e.Name()] {
			continue
		}
		dir := filepath.Join(segmentsPath, e.Name())
		if err := os.RemoveAll(dir); err != nil {
			log.Warn("segment cleanup: failed to remove orphan dir", "path", dir, "err", err)
			continue
		}
		log.Info("segment cleanup: removed orphaned HLS segments", "camera_id", e.Name())
		removed++
	}
	return removed
}
