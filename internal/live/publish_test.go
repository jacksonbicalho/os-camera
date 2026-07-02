package live

import "testing"

func TestShouldPublish(t *testing.T) {
	cases := []struct {
		codec     string
		transport string
		want      bool
	}{
		{"h264", "auto", true},
		{"h264", "webrtc", true},
		{"h264", "", true},   // empty transport defaults to publishing
		{"h264", "hls", false}, // forced HLS: no publisher
		{"hevc", "auto", false},
		{"h265", "webrtc", false},
		{"", "auto", false},
	}
	for _, c := range cases {
		if got := ShouldPublish(c.codec, c.transport); got != c.want {
			t.Errorf("ShouldPublish(%q, %q) = %v, want %v", c.codec, c.transport, got, c.want)
		}
	}
}

func TestShouldRunHLS(t *testing.T) {
	cases := []struct {
		codec     string
		transport string
		want      bool
	}{
		{"h264", "auto", true},    // fallback kept
		{"h264", "hls", true},     // HLS-only
		{"h264", "webrtc", false}, // WebRTC forced and viable → no HLS, no .ts
		{"hevc", "webrtc", true},  // WebRTC can't play H.265 → HLS stays
		{"", "webrtc", true},      // codec unknown → keep HLS (safe)
		{"hevc", "auto", true},
		{"h264", "", true}, // empty transport → keep HLS
	}
	for _, c := range cases {
		if got := ShouldRunHLS(c.codec, c.transport); got != c.want {
			t.Errorf("ShouldRunHLS(%q, %q) = %v, want %v", c.codec, c.transport, got, c.want)
		}
	}
}
