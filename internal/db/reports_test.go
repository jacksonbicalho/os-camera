package db_test

import (
	"testing"
	"time"

	"camera/internal/db"
)

func TestAggregateMotionEvents(t *testing.T) {
	database := openTestDB(t)
	ensureCamera(t, database, "cam1")
	ensureCamera(t, database, "cam2")

	base := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	mk := func(cam string, at time.Time, label string) {
		if err := db.InsertMotionEvent(database, db.MotionEvent{CameraID: cam, OccurredAt: at, Score: 0.5, Label: label}); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	mk("cam1", base, "pessoa")
	mk("cam1", base.Add(time.Hour), "")           // mesmo dia
	mk("cam2", base.Add(24*time.Hour), "carro")   // dia seguinte
	mk("cam1", base.Add(72*time.Hour), "pessoa")  // fora do range (depois do `to`)

	from := time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 6, 22, 0, 0, 0, 0, time.UTC) // exclusivo: pega 20 e 21

	rep, err := db.AggregateMotionEvents(database, from, to)
	if err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	if rep.Total != 3 {
		t.Fatalf("total = %d, want 3", rep.Total)
	}
	if len(rep.ByDay) != 2 || rep.ByDay[0].Day != "2026-06-20" || rep.ByDay[0].Count != 2 || rep.ByDay[1].Day != "2026-06-21" || rep.ByDay[1].Count != 1 {
		t.Errorf("by_day = %+v", rep.ByDay)
	}
	if rep.ByCamera["cam1"] != 2 || rep.ByCamera["cam2"] != 1 {
		t.Errorf("by_camera = %+v", rep.ByCamera)
	}
	if rep.ByLabel["pessoa"] != 1 || rep.ByLabel["carro"] != 1 || rep.ByLabel[""] != 1 {
		t.Errorf("by_label = %+v", rep.ByLabel)
	}
}
