package main

import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestInitWizardDefaultInputs(t *testing.T) {
	lines := []string{
		"",         // port: 8080
		"",         // db_path
		"",         // segments_path
		"",         // storage path
		"",         // timezone: America/Sao_Paulo
		"",         // log output: stdout (no rotation prompts follow)
		"",         // admin username: admin
		"changeme", // admin password
	}
	input := strings.Join(lines, "\n") + "\n"

	yaml, err := initWizard(strings.NewReader(input), io.Discard)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(yaml, "output: stdout") {
		t.Errorf("YAML missing %q\n\nGot:\n%s", "output: stdout", yaml)
	}

	var dbPath, segPath, storagePath string
	if os.Getuid() == 0 {
		dbPath = "db_path: /var/camera/data/camera.db"
		segPath = "segments_path: /var/camera/data/hls"
		storagePath = "path: /var/camera/data/recordings"
	} else {
		dbPath = "db_path: ./camera.db"
		segPath = "segments_path: ./hls"
		storagePath = "path: ./recordings"
	}

	wants := []string{
		"port: 8080",
		dbPath,
		segPath,
		storagePath,
		"timezone: America/Sao_Paulo",
		"username: admin",
		"password: changeme",
	}
	for _, want := range wants {
		if !strings.Contains(yaml, want) {
			t.Errorf("YAML missing %q\n\nGot:\n%s", want, yaml)
		}
	}

	// Storage settings must NOT appear in the YAML — they live only in the DB.
	for _, mustNot := range []string{
		"with_motion_minutes", "without_motion_minutes",
		"max_size_gb", "warn_percent", "interval_minutes",
		"cameras:", "motion:", "username: master",
	} {
		if strings.Contains(yaml, mustNot) {
			t.Errorf("YAML should not contain %q\n\nGot:\n%s", mustNot, yaml)
		}
	}
}

func TestInitWizardCustomValues(t *testing.T) {
	lines := []string{
		"9000",           // port
		"/var/camera.db", // db_path
		"/var/hls",       // segments_path
		"/mnt/cams",      // storage path
		"America/Recife", // timezone
		"",               // log output: stdout
		"master",         // admin username
		"s3cr3t!",        // admin password (has special char)
	}
	input := strings.Join(lines, "\n") + "\n"

	yaml, err := initWizard(strings.NewReader(input), io.Discard)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wants := []string{
		"port: 9000",
		"db_path: /var/camera.db",
		"segments_path: /var/hls",
		"path: /mnt/cams",
		"timezone: America/Recife",
		"username: master",
		`password: "s3cr3t!"`,
	}
	for _, want := range wants {
		if !strings.Contains(yaml, want) {
			t.Errorf("YAML missing %q\n\nGot:\n%s", want, yaml)
		}
	}
}

func TestInitWizardNoCamerasIsNotError(t *testing.T) {
	lines := []string{
		"", // port
		"", // db_path
		"", // segments_path
		"", // storage path
		"", // timezone
		"", // log output: stdout
		"", // admin username
		"", // admin password (empty = default "changeme")
	}
	input := strings.Join(lines, "\n") + "\n"

	_, err := initWizard(strings.NewReader(input), io.Discard)
	if err != nil {
		t.Fatalf("wizard with no cameras should not return error, got: %v", err)
	}
}

func TestInitWizardLogFileEmitsRotation(t *testing.T) {
	lines := []string{
		"",                // port
		"",                // db_path
		"",                // segments_path
		"",                // storage path
		"",                // timezone
		"file",            // log output: file → rotation prompts follow
		"/var/log/camera", // log path
		"25",              // max_size_mb
		"7",               // max_age_days
		"3",               // max_backups
		"n",               // compress? → false
		"",                // admin username
		"",                // admin password
	}
	input := strings.Join(lines, "\n") + "\n"

	yaml, err := initWizard(strings.NewReader(input), io.Discard)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wants := []string{
		"output: file",
		"path: /var/log/camera",
		"max_size_mb: 25",
		"max_age_days: 7",
		"max_backups: 3",
		"compress: false",
	}
	for _, want := range wants {
		if !strings.Contains(yaml, want) {
			t.Errorf("YAML missing %q\n\nGot:\n%s", want, yaml)
		}
	}
}
