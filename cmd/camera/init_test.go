package main

import (
	"io"
	"strings"
	"testing"
)

func TestInitWizardDefaultInputs(t *testing.T) {
	lines := []string{
		"",         // port: 8080
		"",         // db_path: /var/camera/data/camera.db
		"",         // segments_path: /var/camera/data/hls
		"",         // hls_dvr: 0
		"",         // storage path: /var/camera/data/recordings
		"",         // with_motion_minutes: 10080
		"",         // without_motion_minutes: 1440
		"",         // max_size: 10
		"",         // warn_percent: 70
		"",         // timezone: America/Sao_Paulo
		"",         // admin username: admin
		"changeme", // admin password
	}
	input := strings.Join(lines, "\n") + "\n"

	yaml, err := initWizard(strings.NewReader(input), io.Discard)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wants := []string{
		"port: 8080",
		"db_path: /var/camera/data/camera.db",
		"segments_path: /var/camera/data/hls",
		"hls_dvr_seconds: 0",
		"path: /var/camera/data/recordings",
		"with_motion_minutes: 10080",
		"without_motion_minutes: 1440",
		"max_size_gb: 10.0",
		"warn_percent: 70.0",
		"timezone: America/Sao_Paulo",
		"username: admin",
		"password: changeme",
	}
	for _, want := range wants {
		if !strings.Contains(yaml, want) {
			t.Errorf("YAML missing %q\n\nGot:\n%s", want, yaml)
		}
	}

	// Must NOT contain old-format camera/motion sections
	for _, must_not := range []string{"cameras:", "motion:", "username: master"} {
		if strings.Contains(yaml, must_not) {
			t.Errorf("YAML should not contain %q\n\nGot:\n%s", must_not, yaml)
		}
	}
}

func TestInitWizardCustomValues(t *testing.T) {
	lines := []string{
		"9000",           // port
		"/var/camera.db", // db_path
		"/var/hls",       // segments_path
		"1200",           // hls_dvr
		"/mnt/cams",      // storage path
		"10080",          // with_motion_minutes
		"2880",           // without_motion_minutes
		"50",             // max_size_gb
		"80",             // warn_percent
		"America/Recife", // timezone
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
		"hls_dvr_seconds: 1200",
		"path: /mnt/cams",
		"with_motion_minutes: 10080",
		"without_motion_minutes: 2880",
		"max_size_gb: 50.0",
		"warn_percent: 80.0",
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
		"", // hls_dvr
		"", // storage path
		"", // with_motion_minutes
		"", // without_motion_minutes
		"", // max_size_gb
		"", // warn_percent
		"", // timezone
		"", // admin username
		"", // admin password (empty = default "changeme")
	}
	input := strings.Join(lines, "\n") + "\n"

	_, err := initWizard(strings.NewReader(input), io.Discard)
	if err != nil {
		t.Fatalf("wizard with no cameras should not return error, got: %v", err)
	}
}
