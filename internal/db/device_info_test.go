package db_test

import (
	"testing"

	"camera/internal/db"
)

func sampleValues() map[string]string {
	return map[string]string{
		"collector":            "dahua",
		"vendor":               "IntelBras",
		"model":                "iM5",
		"serial":               "6L08ABCDEF",
		"ntp.enabled":          "true",
		"timezone":             "22",
		"stream.main.gop":      "40",
		"raw.table.NTP.Enable": "true",
	}
}

func TestSaveAndGetDeviceInfo(t *testing.T) {
	database := openTestDB(t)
	ensureCamera(t, database, "cam1")

	if err := db.SaveDeviceInfo(database, "cam1", sampleValues()); err != nil {
		t.Fatalf("SaveDeviceInfo: %v", err)
	}

	got, collectedAt, ok, err := db.GetDeviceInfo(database, "cam1")
	if err != nil {
		t.Fatalf("GetDeviceInfo: %v", err)
	}
	if !ok {
		t.Fatal("ok=false, want true")
	}
	if collectedAt.IsZero() {
		t.Error("collectedAt is zero")
	}
	if got["model"] != "iM5" || got["ntp.enabled"] != "true" || got["stream.main.gop"] != "40" {
		t.Errorf("values mismatch: %v", got)
	}
	if got["raw.table.NTP.Enable"] != "true" {
		t.Errorf("raw key not round-tripped: %v", got)
	}
}

func TestGetDeviceInfoMissing(t *testing.T) {
	database := openTestDB(t)
	_, _, ok, err := db.GetDeviceInfo(database, "nope")
	if err != nil {
		t.Fatalf("GetDeviceInfo: %v", err)
	}
	if ok {
		t.Errorf("ok=true for missing camera, want false")
	}
}

func TestSaveDeviceInfoReplacesSnapshot(t *testing.T) {
	database := openTestDB(t)
	ensureCamera(t, database, "cam1")

	if err := db.SaveDeviceInfo(database, "cam1", sampleValues()); err != nil {
		t.Fatalf("SaveDeviceInfo first: %v", err)
	}
	// Second snapshot changes a value and drops a key.
	if err := db.SaveDeviceInfo(database, "cam1", map[string]string{
		"collector": "dahua",
		"model":     "iM5-SC",
	}); err != nil {
		t.Fatalf("SaveDeviceInfo second: %v", err)
	}

	got, _, _, err := db.GetDeviceInfo(database, "cam1")
	if err != nil {
		t.Fatalf("GetDeviceInfo: %v", err)
	}
	if got["model"] != "iM5-SC" {
		t.Errorf("model = %q, want replaced iM5-SC", got["model"])
	}
	if _, exists := got["serial"]; exists {
		t.Errorf("stale key 'serial' survived snapshot replacement: %v", got)
	}
}
