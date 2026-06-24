package db_test

import (
	"testing"
	"time"

	"camera/internal/db"
)

func TestListRecordingsInRange(t *testing.T) {
	database := openTestDB(t)
	ensureCamera(t, database, "cam1")
	ensureCamera(t, database, "cam2")

	base := time.Date(2026, 6, 23, 10, 0, 0, 0, time.UTC)
	mk := func(cam string, at time.Time, path string, motion bool) {
		if err := db.InsertRecording(database, db.Recording{CameraID: cam, StartedAt: at, Path: path, HasMotion: motion}); err != nil {
			t.Fatal(err)
		}
	}
	mk("cam1", base, "/r/cam1/a.mp4", false)                   // dentro
	mk("cam1", base.Add(5*time.Minute), "/r/cam1/b.mp4", true) // dentro, com movimento
	mk("cam1", base.Add(-2*time.Hour), "/r/cam1/old.mp4", true) // fora (antes do start)
	mk("cam2", base.Add(time.Minute), "/r/cam2/c.mp4", true)    // outra câmera

	start := base.Add(-time.Hour)
	end := base.Add(time.Hour)

	// cam1, sem filtro de movimento → as 2 de dentro, desc por started_at
	recs, err := db.ListRecordingsInRange(database, []string{"cam1"}, start, end, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 2 {
		t.Fatalf("esperava 2 gravações, got %d: %+v", len(recs), recs)
	}
	if recs[0].Path != "/r/cam1/b.mp4" || recs[1].Path != "/r/cam1/a.mp4" {
		t.Errorf("ordem desc por started_at incorreta: %+v", recs)
	}

	// motionOnly → só a com has_motion
	mrecs, err := db.ListRecordingsInRange(database, []string{"cam1"}, start, end, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(mrecs) != 1 || mrecs[0].Path != "/r/cam1/b.mp4" {
		t.Errorf("motionOnly esperava só a com movimento, got %+v", mrecs)
	}

	// multi-câmera
	all, err := db.ListRecordingsInRange(database, []string{"cam1", "cam2"}, start, end, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Errorf("multi-câmera esperava 3, got %d: %+v", len(all), all)
	}

	// lista de câmeras vazia → vazio
	none, err := db.ListRecordingsInRange(database, nil, start, end, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(none) != 0 {
		t.Errorf("sem câmeras esperava 0, got %d", len(none))
	}
}
