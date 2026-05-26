package db_test

import (
	"testing"

	"camera/internal/db"
)

func TestSetAndGetConfig(t *testing.T) {
	database := openTestDB(t)

	if err := db.SetConfig(database, "server.port", "9090"); err != nil {
		t.Fatalf("SetConfig: %v", err)
	}

	val, err := db.GetConfig(database, "server.port")
	if err != nil {
		t.Fatalf("GetConfig: %v", err)
	}
	if val != "9090" {
		t.Errorf("got %q, want %q", val, "9090")
	}
}

func TestSetConfig_Upsert(t *testing.T) {
	database := openTestDB(t)

	if err := db.SetConfig(database, "key", "v1"); err != nil {
		t.Fatalf("SetConfig v1: %v", err)
	}
	if err := db.SetConfig(database, "key", "v2"); err != nil {
		t.Fatalf("SetConfig v2: %v", err)
	}

	val, _ := db.GetConfig(database, "key")
	if val != "v2" {
		t.Errorf("esperava v2, got %q", val)
	}
}

func TestGetConfig_Missing(t *testing.T) {
	database := openTestDB(t)

	_, err := db.GetConfig(database, "chave-inexistente")
	if err == nil {
		t.Error("esperava erro para chave inexistente")
	}
}

func TestStorageSettingsFromDB_FallsBackToDefaults(t *testing.T) {
	database := openTestDB(t)

	got := db.StorageSettingsFromDB(database)

	if got.WithMotionMinutes != db.DefaultStorageSettings.WithMotionMinutes {
		t.Errorf("WithMotionMinutes: got %d, want %d", got.WithMotionMinutes, db.DefaultStorageSettings.WithMotionMinutes)
	}
	if got.WithoutMotionMinutes != db.DefaultStorageSettings.WithoutMotionMinutes {
		t.Errorf("WithoutMotionMinutes: got %d, want %d", got.WithoutMotionMinutes, db.DefaultStorageSettings.WithoutMotionMinutes)
	}
	if got.MaxSizeGB != db.DefaultStorageSettings.MaxSizeGB {
		t.Errorf("MaxSizeGB: got %f, want %f", got.MaxSizeGB, db.DefaultStorageSettings.MaxSizeGB)
	}
}

func TestStorageSettingsFromDB_DBOverridesDefaults(t *testing.T) {
	database := openTestDB(t)

	_ = db.SetConfig(database, "storage.with_motion_minutes", "120")
	_ = db.SetConfig(database, "storage.without_motion_minutes", "45")
	_ = db.SetConfig(database, "storage.interval_minutes", "20")
	_ = db.SetConfig(database, "storage.max_size_gb", "50")
	_ = db.SetConfig(database, "storage.warn_percent", "90")

	got := db.StorageSettingsFromDB(database)

	if got.WithMotionMinutes != 120 {
		t.Errorf("WithMotionMinutes: got %d, want 120", got.WithMotionMinutes)
	}
	if got.WithoutMotionMinutes != 45 {
		t.Errorf("WithoutMotionMinutes: got %d, want 45", got.WithoutMotionMinutes)
	}
	if got.IntervalMinutes != 20 {
		t.Errorf("IntervalMinutes: got %d, want 20", got.IntervalMinutes)
	}
	if got.MaxSizeGB != 50 {
		t.Errorf("MaxSizeGB: got %f, want 50", got.MaxSizeGB)
	}
	if got.WarnPercent != 90 {
		t.Errorf("WarnPercent: got %f, want 90", got.WarnPercent)
	}
}

func TestStorageSettingsFromDB_NilDB(t *testing.T) {
	got := db.StorageSettingsFromDB(nil)

	if got.MaxSizeGB != db.DefaultStorageSettings.MaxSizeGB {
		t.Errorf("MaxSizeGB: got %f, want %f", got.MaxSizeGB, db.DefaultStorageSettings.MaxSizeGB)
	}
}

func TestEnsureStorageDefaults_WritesDefaults(t *testing.T) {
	database := openTestDB(t)

	if err := db.EnsureStorageDefaults(database); err != nil {
		t.Fatalf("EnsureStorageDefaults: %v", err)
	}

	got := db.StorageSettingsFromDB(database)
	if got.WithMotionMinutes != db.DefaultStorageSettings.WithMotionMinutes {
		t.Errorf("WithMotionMinutes: got %d, want %d", got.WithMotionMinutes, db.DefaultStorageSettings.WithMotionMinutes)
	}
	if got.IntervalMinutes != db.DefaultStorageSettings.IntervalMinutes {
		t.Errorf("IntervalMinutes: got %d, want %d", got.IntervalMinutes, db.DefaultStorageSettings.IntervalMinutes)
	}
}

func TestEnsureStorageDefaults_DoesNotOverrideExisting(t *testing.T) {
	database := openTestDB(t)

	_ = db.SetConfig(database, "storage.max_size_gb", "99")

	if err := db.EnsureStorageDefaults(database); err != nil {
		t.Fatalf("EnsureStorageDefaults: %v", err)
	}

	got := db.StorageSettingsFromDB(database)
	if got.MaxSizeGB != 99 {
		t.Errorf("MaxSizeGB: got %f, want 99 (existing value must not be overwritten)", got.MaxSizeGB)
	}
}

func TestGetAllConfig(t *testing.T) {
	database := openTestDB(t)

	pairs := map[string]string{
		"a": "1",
		"b": "2",
		"c": "3",
	}
	for k, v := range pairs {
		if err := db.SetConfig(database, k, v); err != nil {
			t.Fatalf("SetConfig %s: %v", k, err)
		}
	}

	all, err := db.GetAllConfig(database)
	if err != nil {
		t.Fatalf("GetAllConfig: %v", err)
	}
	if len(all) != len(pairs) {
		t.Errorf("esperava %d entradas, got %d", len(pairs), len(all))
	}
	for k, want := range pairs {
		if got := all[k]; got != want {
			t.Errorf("all[%q]: got %q, want %q", k, got, want)
		}
	}
}
