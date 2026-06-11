package db_test

import (
	"testing"

	"camera/internal/db"
)

func TestUserTheme_DefaultIsDark(t *testing.T) {
	d := openTestDB(t)
	u := mkUser(t, d, "u1")

	th, err := db.GetUserTheme(d, u)
	if err != nil {
		t.Fatalf("GetUserTheme: %v", err)
	}
	if th != "dark" {
		t.Errorf("expected default theme 'dark', got %q", th)
	}
}

func TestUserTheme_SetAndGet(t *testing.T) {
	d := openTestDB(t)
	u := mkUser(t, d, "u1")

	if err := db.SetUserTheme(d, u, "light"); err != nil {
		t.Fatalf("SetUserTheme: %v", err)
	}
	th, err := db.GetUserTheme(d, u)
	if err != nil {
		t.Fatalf("GetUserTheme: %v", err)
	}
	if th != "light" {
		t.Errorf("expected 'light', got %q", th)
	}
}
