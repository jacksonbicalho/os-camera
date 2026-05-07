package main

import (
	"io"
	"strings"
	"testing"
)

func TestInitWizardDefaultInputs(t *testing.T) {
	// Simulate user accepting all defaults, one camera, then empty ID to finish.
	lines := []string{
		"",                              // port: 8080
		"",                              // username: master
		"",                              // password: ""
		"",                              // segments_path: /tmp/hls
		"",                              // hls_dvr: 0
		"",                              // storage path: /data/recordings
		"",                              // retention: 43200
		"",                              // max_size: 10
		"",                              // warn_percent: 70
		"",                              // timezone: America/Sao_Paulo
		"",                              // motion enabled: n
		"",                              // threshold: 0.02
		"",                              // fps: 2
		"",                              // cooldown: 30
		"garage",                        // camera id
		"rtsp://192.168.1.10:554/stream", // rtsp url
		"",                              // has_audio: auto
		"",                              // motion: global
		"",                              // next camera id: empty = stop
	}
	input := strings.Join(lines, "\n") + "\n"

	yaml, err := initWizard(strings.NewReader(input), io.Discard)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wants := []string{
		"port: 8080",
		"username: master",
		"password: \"\"",
		"segments_path: /tmp/hls",
		"hls_dvr_seconds: 0",
		"path: /data/recordings",
		"retention_minutes: 43200",
		"max_size_gb: 10.0",
		"warn_percent: 70.0",
		"timezone: America/Sao_Paulo",
		"enabled: false",
		"threshold: 0.02",
		"fps: 2",
		"cooldown_seconds: 30",
		"id: garage",
		"rtsp_url: rtsp://192.168.1.10:554/stream",
	}
	for _, want := range wants {
		if !strings.Contains(yaml, want) {
			t.Errorf("YAML missing %q\n\nGot:\n%s", want, yaml)
		}
	}
}

func TestInitWizardCustomValues(t *testing.T) {
	lines := []string{
		"9000",                           // port
		"admin",                          // username
		"s3cr3t",                         // password
		"/var/hls",                       // segments_path
		"1200",                           // hls_dvr
		"/mnt/cams",                      // storage path
		"10080",                          // retention (7 days)
		"50",                             // max_size_gb
		"80",                             // warn_percent
		"America/Recife",                 // timezone
		"s",                              // motion enabled
		"0.03",                           // threshold
		"4",                              // fps
		"60",                             // cooldown
		"entrada",                        // camera 1 id
		"rtsp://10.0.0.1:554/ch0",        // rtsp
		"n",                              // has_audio: false
		"s",                              // motion: sim (override)
		"0.05",                           // threshold
		"3",                              // fps
		"45",                             // cooldown
		"quintal",                        // camera 2 id
		"rtsp://10.0.0.2:554/ch0",        // rtsp
		"s",                              // has_audio: true
		"n",                              // motion: disabled for this cam
		"",                               // stop
	}
	input := strings.Join(lines, "\n") + "\n"

	yaml, err := initWizard(strings.NewReader(input), io.Discard)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wants := []string{
		"port: 9000",
		"username: admin",
		"password: s3cr3t",
		"segments_path: /var/hls",
		"hls_dvr_seconds: 1200",
		"path: /mnt/cams",
		"retention_minutes: 10080",
		"max_size_gb: 50.0",
		"warn_percent: 80.0",
		"timezone: America/Recife",
		"threshold: 0.03",
		"fps: 4",
		"cooldown_seconds: 60",
		"id: entrada",
		"has_audio: false",
		"      threshold: 0.05",
		"      fps: 3",
		"      cooldown_seconds: 45",
		"id: quintal",
		"has_audio: true",
		"      enabled: false",
	}
	for _, want := range wants {
		if !strings.Contains(yaml, want) {
			t.Errorf("YAML missing %q\n\nGot:\n%s", want, yaml)
		}
	}
}

func TestInitWizardNoCamerasReturnsError(t *testing.T) {
	// All defaults, then immediately empty camera ID.
	lines := []string{
		"", "", "", "", "",
		"", "", "", "",
		"",
		"", "", "", "",
		"", // camera id: empty = no cameras
	}
	input := strings.Join(lines, "\n") + "\n"

	_, err := initWizard(strings.NewReader(input), io.Discard)
	if err == nil {
		t.Fatal("expected error for no cameras, got nil")
	}
}
