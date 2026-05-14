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
