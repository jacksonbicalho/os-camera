package live

// ShouldPublish reports whether a WebRTC publisher should run for a camera with
// the given video codec and live transport preference. WebRTC in browsers only
// plays H.264, and the "hls" preference forces HLS by not publishing at all
// (the client then gets a 409 and falls back). "auto"/"webrtc"/"" all publish
// when the codec is H.264.
func ShouldPublish(videoCodec, transport string) bool {
	return videoCodec == "h264" && transport != "hls"
}
