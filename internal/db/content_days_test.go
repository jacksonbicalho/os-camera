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

	days, err := db.ContentDays(d, "cam1", loc)
	if err != nil {
		t.Fatalf("ContentDays: %v", err)
	}

	want := []string{"2026-06-19", "2026-06-20"}
	if len(days) != len(want) {
		t.Fatalf("expected %v, got %v", want, days)
	}
	for i := range want {
		if days[i] != want[i] {
			t.Fatalf("expected %v, got %v", want, days)
		}
	}
}

func TestContentDays_Empty(t *testing.T) {
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.Close()

	days, err := db.ContentDays(d, "nope", time.UTC)
	if err != nil {
		t.Fatalf("ContentDays: %v", err)
	}
	if len(days) != 0 {
		t.Fatalf("expected no days, got %v", days)
	}
}
