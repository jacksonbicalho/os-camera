package db_test

import (
	"testing"
	"time"

	"camera/internal/config"
	"camera/internal/db"
)

func TestContentDays_UnionAndLocalOffset(t *testing.T) {
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.Close()

	for _, cid := range []string{"cam1", "cam2"} {
		if _, err := db.CreateCamera(d, config.CameraConfig{ID: cid, Name: cid, RTSPURL: "rtsp://x"}, nil); err != nil {
			t.Fatalf("create camera %s: %v", cid, err)
		}
	}

	loc := time.FixedZone("BRT", -3*3600) // -03:00, sem DST

	mustUTC := func(s string) time.Time {
		ts, err := time.Parse(time.RFC3339, s)
		if err != nil {
			t.Fatalf("parse %s: %v", s, err)
		}
		return ts
	}

	// Evento às 02:00Z → local 23:00 do dia anterior → 2026-06-19.
	if err := db.InsertMotionEvent(d, db.MotionEvent{CameraID: "cam1", OccurredAt: mustUTC("2026-06-20T02:00:00Z")}); err != nil {
		t.Fatalf("insert event: %v", err)
	}
	// Gravação às 12:00Z → local 09:00 → 2026-06-20.
	if err := db.InsertRecording(d, db.Recording{CameraID: "cam1", StartedAt: mustUTC("2026-06-20T12:00:00Z"), Path: "r1.mp4"}); err != nil {
		t.Fatalf("insert rec: %v", err)
	}
	// Gravação + evento no mesmo dia local (2026-06-20) → não duplica.
	if err := db.InsertRecording(d, db.Recording{CameraID: "cam1", StartedAt: mustUTC("2026-06-20T15:00:00Z"), Path: "r2.mp4"}); err != nil {
		t.Fatalf("insert rec2: %v", err)
	}
	if err := db.InsertMotionEvent(d, db.MotionEvent{CameraID: "cam1", OccurredAt: mustUTC("2026-06-20T16:00:00Z")}); err != nil {
		t.Fatalf("insert event2: %v", err)
	}
	// Outra câmera não deve aparecer.
	if err := db.InsertRecording(d, db.Recording{CameraID: "cam2", StartedAt: mustUTC("2026-06-25T12:00:00Z"), Path: "other.mp4"}); err != nil {
		t.Fatalf("insert other: %v", err)
	}

	eq := func(got, want []string) {
		t.Helper()
		if len(got) != len(want) {
			t.Fatalf("expected %v, got %v", want, got)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("expected %v, got %v", want, got)
			}
		}
	}

	days, err := db.ContentDays(d, "cam1", loc, db.ContentAll)
	if err != nil {
		t.Fatalf("ContentDays all: %v", err)
	}
	eq(days, []string{"2026-06-19", "2026-06-20"})

	// recordings-only: só o dia da gravação (2026-06-20).
	recDays, err := db.ContentDays(d, "cam1", loc, db.ContentRecordings)
	if err != nil {
		t.Fatalf("ContentDays recordings: %v", err)
	}
	eq(recDays, []string{"2026-06-20"})

	// events-only: o evento das 02:00Z (2026-06-19) + o das 16:00Z (2026-06-20).
	evDays, err := db.ContentDays(d, "cam1", loc, db.ContentEvents)
	if err != nil {
		t.Fatalf("ContentDays events: %v", err)
	}
	eq(evDays, []string{"2026-06-19", "2026-06-20"})

	// Multi-câmera: une cam1 (2026-06-20) e cam2 (2026-06-25) por gravação.
	multi, err := db.ContentDaysMulti(d, []string{"cam1", "cam2"}, loc, db.ContentRecordings)
	if err != nil {
		t.Fatalf("ContentDaysMulti: %v", err)
	}
	eq(multi, []string{"2026-06-20", "2026-06-25"})
}

func TestContentDays_Empty(t *testing.T) {
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.Close()

	days, err := db.ContentDays(d, "nope", time.UTC, db.ContentAll)
	if err != nil {
		t.Fatalf("ContentDays: %v", err)
	}
	if len(days) != 0 {
		t.Fatalf("expected no days, got %v", days)
	}
}
