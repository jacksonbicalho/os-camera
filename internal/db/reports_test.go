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
	mk("cam1", base.Add(time.Hour), "")          // mesmo dia
	mk("cam2", base.Add(24*time.Hour), "carro")  // outra câmera → fora do escopo
	mk("cam1", base.Add(72*time.Hour), "pessoa") // fora do range (depois do `to`)

	// Transições de estado de cam1 entram no relatório (categoria "estados").
	c1, err := db.CreateStateClassifier(database, makeClassifier("cam1"))
	if err != nil {
		t.Fatalf("classifier: %v", err)
	}
	insertTransition(t, database, c1, "aberto", 0.9, "/x/a.jpg", "2026-06-20 12:00:00")  // dia 20
	insertTransition(t, database, c1, "fechado", 0.9, "/x/b.jpg", "2026-06-21 09:00:00") // dia 21
	insertTransition(t, database, c1, "aberto", 0.9, "/x/c.jpg", "2026-06-25 09:00:00")  // fora do range

	from := time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 6, 22, 0, 0, 0, 0, time.UTC) // exclusivo: pega 20 e 21

	rep, err := db.AggregateMotionEvents(database, from, to, "cam1")
	if err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	// 2 motion (cam1, dia 20) + 2 estados (cam1, dias 20 e 21) = 4; cam2 fora do escopo
	if rep.Total != 4 {
		t.Fatalf("total = %d, want 4", rep.Total)
	}
	if len(rep.ByDay) != 2 || rep.ByDay[0].Day != "2026-06-20" || rep.ByDay[0].Count != 3 || rep.ByDay[1].Day != "2026-06-21" || rep.ByDay[1].Count != 1 {
		t.Errorf("by_day = %+v", rep.ByDay)
	}
	// dia 20: 1 pessoa + 1 movimento (label vazio) + 1 estado
	if rep.ByDay[0].ByCategory["pessoa"] != 1 || rep.ByDay[0].ByCategory["movimento"] != 1 || rep.ByDay[0].ByCategory["estados"] != 1 {
		t.Errorf("by_day[0].by_category = %+v", rep.ByDay[0].ByCategory)
	}
	// dia 21: só 1 estado
	if rep.ByDay[1].ByCategory["estados"] != 1 || len(rep.ByDay[1].ByCategory) != 1 {
		t.Errorf("by_day[1].by_category = %+v", rep.ByDay[1].ByCategory)
	}
	if rep.ByCategory["estados"] != 2 {
		t.Errorf("by_category[estados] = %d, want 2 (%+v)", rep.ByCategory["estados"], rep.ByCategory)
	}
	if rep.ByLabel["pessoa"] != 1 || rep.ByLabel[""] != 1 {
		t.Errorf("by_label = %+v", rep.ByLabel)
	}
	if _, ok := rep.ByLabel["carro"]; ok {
		t.Errorf("by_label não deveria conter label de cam2: %+v", rep.ByLabel)
	}
}
