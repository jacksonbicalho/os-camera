package server

import "testing"

func TestRestoreMaskedRTSPPassword_RestoresFromExisting(t *testing.T) {
	submitted := "rtsp://admin:xxxxx@192.168.1.16:554/cam/realmonitor?channel=1&subtype=0"
	existing := "rtsp://admin:s3cr3t@192.168.1.16:554/cam/realmonitor?channel=1&subtype=0"
	got := restoreMaskedRTSPPassword(submitted, existing)
	want := "rtsp://admin:s3cr3t@192.168.1.16:554/cam/realmonitor?channel=1&subtype=0"
	if got != want {
		t.Fatalf("restoreMaskedRTSPPassword = %q, want %q", got, want)
	}
}

func TestRestoreMaskedRTSPPassword_KeepsSubmittedHostChanges(t *testing.T) {
	// User changed the host in the form but left the (masked) password: keep the
	// new host, restore the old password.
	submitted := "rtsp://admin:xxxxx@10.0.0.5:554/cam/realmonitor?channel=1&subtype=0"
	existing := "rtsp://admin:s3cr3t@192.168.1.16:554/cam/realmonitor?channel=1&subtype=0"
	got := restoreMaskedRTSPPassword(submitted, existing)
	want := "rtsp://admin:s3cr3t@10.0.0.5:554/cam/realmonitor?channel=1&subtype=0"
	if got != want {
		t.Fatalf("restoreMaskedRTSPPassword = %q, want %q", got, want)
	}
}

func TestRestoreMaskedRTSPPassword_LeavesUnmaskedUnchanged(t *testing.T) {
	submitted := "rtsp://admin:realpass@192.168.1.16:554/cam/realmonitor?channel=1&subtype=0"
	if got := restoreMaskedRTSPPassword(submitted, "rtsp://admin:other@host/x"); got != submitted {
		t.Fatalf("restoreMaskedRTSPPassword changed an unmasked URL: %q", got)
	}
}
