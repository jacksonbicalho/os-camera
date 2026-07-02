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
