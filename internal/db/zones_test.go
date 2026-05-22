package db_test

import (
	"testing"

	"camera/internal/db"
	"camera/internal/zones"
)

func TestGetZonesEmptyByDefault(t *testing.T) {
	database := openTestDB(t)
	c := makeCamera("cam1")
	c.ID = "cam1"
	if _, err := db.CreateCamera(database, c, nil); err != nil {
		t.Fatal(err)
	}
	got, err := db.GetZones(database, "cam1")
	if err != nil {
		t.Fatalf("GetZones: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty zones, got %v", got)
	}
}

func TestSetAndGetZones(t *testing.T) {
	database := openTestDB(t)
	c := makeCamera("cam1")
	c.ID = "cam1"
	if _, err := db.CreateCamera(database, c, nil); err != nil {
		t.Fatal(err)
	}

	zs := []zones.Zone{
		{X: 0.1, Y: 0.2, W: 0.3, H: 0.4, Type: "exclude", Label: "porta", Color: "#ef4444"},
		{X: 0.5, Y: 0.5, W: 0.2, H: 0.2, Type: "detect", Threshold: 0.05, FPS: 2, Color: "#3b82f6"},
	}
	if err := db.SetZones(database, "cam1", zs); err != nil {
		t.Fatalf("SetZones: %v", err)
	}

	got, err := db.GetZones(database, "cam1")
	if err != nil {
		t.Fatalf("GetZones: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 zones, got %d: %v", len(got), got)
	}
	if got[0].X != 0.1 || got[0].Label != "porta" || got[0].Type != "exclude" || got[0].Color != "#ef4444" {
		t.Errorf("zone 0 mismatch: %+v", got[0])
	}
	if got[1].Type != "detect" || got[1].Threshold != 0.05 || got[1].FPS != 2 || got[1].Color != "#3b82f6" {
		t.Errorf("zone 1 mismatch: %+v", got[1])
	}
}

func TestSetZonesReplacesExisting(t *testing.T) {
	database := openTestDB(t)
	c := makeCamera("cam1")
	c.ID = "cam1"
	if _, err := db.CreateCamera(database, c, nil); err != nil {
		t.Fatal(err)
	}

	db.SetZones(database, "cam1", []zones.Zone{{X: 0.1, Y: 0.1, W: 0.1, H: 0.1}}) //nolint:errcheck
	if err := db.SetZones(database, "cam1", []zones.Zone{}); err != nil {
		t.Fatalf("SetZones empty: %v", err)
	}

	got, _ := db.GetZones(database, "cam1")
	if len(got) != 0 {
		t.Fatalf("expected empty after replace, got %v", got)
	}
}

func TestSetZonesNormalizesEmptyType(t *testing.T) {
	database := openTestDB(t)
	c := makeCamera("cam1")
	c.ID = "cam1"
	if _, err := db.CreateCamera(database, c, nil); err != nil {
		t.Fatal(err)
	}

	if err := db.SetZones(database, "cam1", []zones.Zone{{X: 0.1, Y: 0.1, W: 0.1, H: 0.1, Type: ""}}); err != nil {
		t.Fatal(err)
	}

	got, _ := db.GetZones(database, "cam1")
	if len(got) != 1 || got[0].Type != "exclude" {
		t.Fatalf("expected type=exclude, got %+v", got)
	}
}
