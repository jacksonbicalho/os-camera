package db_test

import (
	"testing"

	"camera/internal/db"
)

func TestDrives_CreateListDelete(t *testing.T) {
	database := openTestDB(t)

	drives, err := db.ListDrives(database)
	if err != nil {
		t.Fatalf("ListDrives: %v", err)
	}
	if len(drives) != 0 {
		t.Fatalf("expected 0 drives, got %d", len(drives))
	}

	created, err := db.InsertDrive(database, db.Drive{
		Name:      "my-s3",
		Type:      "s3",
		Bucket:    "my-bucket",
		Region:    "us-east-1",
		AccessKey: "AKID",
		SecretKey: "SECRET",
	})
	if err != nil {
		t.Fatalf("InsertDrive: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if created.Name != "my-s3" {
		t.Errorf("Name = %q, want %q", created.Name, "my-s3")
	}

	drives, err = db.ListDrives(database)
	if err != nil {
		t.Fatalf("ListDrives: %v", err)
	}
	if len(drives) != 1 {
		t.Fatalf("expected 1 drive, got %d", len(drives))
	}

	if err := db.DeleteDrive(database, created.ID); err != nil {
		t.Fatalf("DeleteDrive: %v", err)
	}
	drives, _ = db.ListDrives(database)
	if len(drives) != 0 {
		t.Fatalf("expected 0 drives after delete, got %d", len(drives))
	}
}

func TestDrives_Update(t *testing.T) {
	database := openTestDB(t)

	dr, err := db.InsertDrive(database, db.Drive{
		Name:      "test",
		Type:      "s3",
		Bucket:    "bucket1",
		AccessKey: "AK",
		SecretKey: "SK",
	})
	if err != nil {
		t.Fatalf("InsertDrive: %v", err)
	}

	dr.Bucket = "bucket2"
	dr.Prefix = "archive/"
	if err := db.UpdateDrive(database, dr); err != nil {
		t.Fatalf("UpdateDrive: %v", err)
	}

	got, err := db.GetDrive(database, dr.ID)
	if err != nil {
		t.Fatalf("GetDrive: %v", err)
	}
	if got.Bucket != "bucket2" {
		t.Errorf("Bucket = %q, want %q", got.Bucket, "bucket2")
	}
	if got.Prefix != "archive/" {
		t.Errorf("Prefix = %q, want %q", got.Prefix, "archive/")
	}
}

func TestRetentionConfig_DefaultsAndUpdate(t *testing.T) {
	database := openTestDB(t)

	configs, err := db.ListRetentionConfigs(database)
	if err != nil {
		t.Fatalf("ListRetentionConfigs: %v", err)
	}
	if len(configs) != 2 {
		t.Fatalf("expected 2 default configs, got %d", len(configs))
	}
	for _, rc := range configs {
		if rc.Action != "delete" {
			t.Errorf("category %q: default action = %q, want %q", rc.Category, rc.Action, "delete")
		}
	}

	dr, _ := db.InsertDrive(database, db.Drive{
		Name:      "s3",
		Type:      "s3",
		Bucket:    "b",
		AccessKey: "AK",
		SecretKey: "SK",
	})

	if err := db.UpdateRetentionConfig(database, db.RetentionConfig{
		Category: "without_motion",
		Action:   "send_to_drive",
		DriveID:  dr.ID,
	}); err != nil {
		t.Fatalf("UpdateRetentionConfig: %v", err)
	}

	configs, _ = db.ListRetentionConfigs(database)
	for _, rc := range configs {
		if rc.Category == "without_motion" {
			if rc.Action != "send_to_drive" {
				t.Errorf("action = %q, want send_to_drive", rc.Action)
			}
			if rc.DriveID != dr.ID {
				t.Errorf("drive_id = %q, want %q", rc.DriveID, dr.ID)
			}
		}
	}
}
