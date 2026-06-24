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

func TestAggregateMotionEventsHourly(t *testing.T) {
	database := openTestDB(t)
	ensureCamera(t, database, "cam1")
	loc := time.FixedZone("BRT", -3*3600) // UTC-3

	mk := func(at time.Time, label string) {
		if err := db.InsertMotionEvent(database, db.MotionEvent{CameraID: "cam1", OccurredAt: at, Score: 0.5, Label: label}); err != nil {
			t.Fatal(err)
		}
	}
	// dia (em BRT) = 2026-06-21 → [03:00Z, +24h)
	mk(time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC), "")       // 09:00 BRT → hora 9, movimento
	mk(time.Date(2026, 6, 21, 12, 30, 0, 0, time.UTC), "pessoa") // 09:30 BRT → hora 9, pessoa
	c1, err := db.CreateStateClassifier(database, makeClassifier("cam1"))
	if err != nil {
		t.Fatal(err)
	}
	insertTransition(t, database, c1, "aberto", 0.9, "/x/a.jpg", "2026-06-21 15:00:00") // 12:00 BRT → hora 12, estados

	from := time.Date(2026, 6, 21, 3, 0, 0, 0, time.UTC)  // 00:00 BRT
	to := time.Date(2026, 6, 22, 3, 0, 0, 0, time.UTC)    // 24:00 BRT

	rep, err := db.AggregateMotionEventsHourly(database, from, to, "cam1", loc)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.ByHour) != 24 {
		t.Fatalf("esperava 24 buckets de hora, got %d", len(rep.ByHour))
	}
	if rep.Total != 3 {
		t.Fatalf("total = %d, want 3", rep.Total)
	}
	if rep.ByHour[9].Hour != 9 || rep.ByHour[9].Count != 2 || rep.ByHour[9].ByCategory["movimento"] != 1 || rep.ByHour[9].ByCategory["pessoa"] != 1 {
		t.Errorf("hora 9 = %+v", rep.ByHour[9])
	}
	if rep.ByHour[12].Count != 1 || rep.ByHour[12].ByCategory["estados"] != 1 {
		t.Errorf("hora 12 (estado) = %+v", rep.ByHour[12])
	}
	if rep.ByHour[0].Count != 0 {
		t.Errorf("hora 0 deveria ser zero: %+v", rep.ByHour[0])
	}
	if rep.ByCategory["estados"] != 1 {
		t.Errorf("by_category[estados] = %d, want 1", rep.ByCategory["estados"])
	}
	if rep.ByLabel[""] != 1 || rep.ByLabel["pessoa"] != 1 {
		t.Errorf("by_label = %+v", rep.ByLabel)
	}
}

func TestAggregateMotionEventsFillsEmptyDays(t *testing.T) {
	database := openTestDB(t)
	ensureCamera(t, database, "cam1")
	// evento só no dia 21
	if err := db.InsertMotionEvent(database, db.MotionEvent{
		CameraID: "cam1", OccurredAt: time.Date(2026, 6, 21, 10, 0, 0, 0, time.UTC), Score: 0.5, Label: "pessoa",
	}); err != nil {
		t.Fatal(err)
	}

	from := time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 6, 23, 0, 0, 0, 0, time.UTC) // janela contínua: 20, 21, 22

	rep, err := db.AggregateMotionEvents(database, from, to, "cam1")
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.ByDay) != 3 {
		t.Fatalf("esperava 3 dias contínuos (20,21,22), got %d: %+v", len(rep.ByDay), rep.ByDay)
	}
	if rep.ByDay[0].Day != "2026-06-20" || rep.ByDay[0].Count != 0 {
		t.Errorf("dia 20 deveria ser zero: %+v", rep.ByDay[0])
	}
	if rep.ByDay[1].Day != "2026-06-21" || rep.ByDay[1].Count != 1 {
		t.Errorf("dia 21 deveria ter 1: %+v", rep.ByDay[1])
	}
	if rep.ByDay[2].Day != "2026-06-22" || rep.ByDay[2].Count != 0 {
		t.Errorf("dia 22 deveria ser zero: %+v", rep.ByDay[2])
	}
}

func TestAggregateMotionEventsDayHour(t *testing.T) {
	database := openTestDB(t)
	ensureCamera(t, database, "cam1")
	ensureCamera(t, database, "cam2")
	loc := time.FixedZone("BRT", -3*3600) // UTC-3

	mk := func(cam string, at time.Time, label string) {
		if err := db.InsertMotionEvent(database, db.MotionEvent{CameraID: cam, OccurredAt: at, Score: 0.5, Label: label}); err != nil {
			t.Fatal(err)
		}
	}
	mk("cam1", time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC), "pessoa") // 09:00 BRT 06-21 → (06-21, 9)
	mk("cam1", time.Date(2026, 6, 21, 12, 30, 0, 0, time.UTC), "")      // 09:30 BRT 06-21 → (06-21, 9)
	mk("cam2", time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC), "carro")  // outra câmera → ignorado

	c1, err := db.CreateStateClassifier(database, makeClassifier("cam1"))
	if err != nil {
		t.Fatal(err)
	}
	insertTransition(t, database, c1, "aberto", 0.9, "/x/a.jpg", "2026-06-22 15:00:00") // 12:00 BRT 06-22 → (06-22, 12)

	from := time.Date(2026, 6, 21, 3, 0, 0, 0, time.UTC) // 00:00 BRT 06-21
	to := time.Date(2026, 6, 24, 3, 0, 0, 0, time.UTC)   // +3 dias (21, 22, 23)

	rep, err := db.AggregateMotionEventsDayHour(database, from, to, "cam1", loc)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Heatmap) != 72 { // 3 dias × 24
		t.Fatalf("esperava 72 células (3×24), got %d", len(rep.Heatmap))
	}
	if rep.Total != 3 {
		t.Fatalf("total = %d, want 3", rep.Total)
	}
	cell := func(date string, hour int) db.DayHourCell {
		for _, c := range rep.Heatmap {
			if c.Date == date && c.Hour == hour {
				return c
			}
		}
		t.Fatalf("célula (%s,%d) ausente", date, hour)
		return db.DayHourCell{}
	}
	if c := cell("2026-06-21", 9); c.Count != 2 {
		t.Errorf("célula 06-21/9h = %+v, want count 2", c)
	}
	if c := cell("2026-06-22", 12); c.Count != 1 {
		t.Errorf("célula 06-22/12h (estado) = %+v, want count 1", c)
	}
	if c := cell("2026-06-23", 3); c.Count != 0 {
		t.Errorf("célula sem evento deveria ser zero: %+v", c)
	}
	// ordenado por data, depois hora → primeiro bloco = 06-21
	if rep.Heatmap[0].Date != "2026-06-21" || rep.Heatmap[0].Hour != 0 {
		t.Errorf("primeira célula = %+v, want (06-21, 0)", rep.Heatmap[0])
	}
	if rep.Heatmap[9].Date != "2026-06-21" || rep.Heatmap[9].Hour != 9 || rep.Heatmap[9].Count != 2 {
		t.Errorf("índice 9 = %+v, want 06-21/9h count 2", rep.Heatmap[9])
	}
}
