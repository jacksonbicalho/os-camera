package storage

import (
	"log/slog"
	"os"

	"camera/internal/db"
)

// CleanOrphanedRecordings deletes incomplete MP4 files left on disk when the
// system stopped mid-recording (ended_at IS NULL in the recordings table).
//
// Must be called at startup before the recorder and cleaner begin so that
// syncRecordings cannot assign ended_at to these files and make them eligible
// for YOLO analysis with a corrupt or incomplete MP4.
//
// Returns the number of recording rows successfully removed from the database.
func CleanOrphanedRecordings(database *db.DB, log *slog.Logger) int {
	orphans, err := db.ListOrphanedRecordings(database)
	if err != nil {
		log.Error("orphan cleanup: query failed", "err", err)
		return 0
	}
	if len(orphans) == 0 {
		return 0
	}
	removed := 0
	for _, r := range orphans {
		if err := os.Remove(r.Path); err != nil && !os.IsNotExist(err) {
			log.Warn("orphan cleanup: failed to delete file", "path", r.Path, "err", err)
		}
		if err := db.DeleteRecording(database, r.Path); err != nil {
			log.Warn("orphan cleanup: failed to delete recording row", "path", r.Path, "err", err)
			continue
		}
		log.Info("orphan cleanup: deleted incomplete recording",
			"path", r.Path,
			"camera_id", r.CameraID,
			"started_at", r.StartedAt,
		)
		removed++
	}
	return removed
}
