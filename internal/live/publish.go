package live

// ShouldPublish reports whether a WebRTC publisher should run for a camera with
// the given video codec and live transport preference. WebRTC in browsers only
// plays H.264, and the "hls" preference forces HLS by not publishing at all
// (the client then gets a 409 and falls back). "auto"/"webrtc"/"" all publish
// when the codec is H.264.
func ShouldPublish(videoCodec, transport string) bool {
	return videoCodec == "h264" && transport != "hls"
}

// ShouldRunHLS reports whether the HLS pipeline should run for a camera with the
// given video codec and live transport preference. HLS is kept unless WebRTC is
// the forced transport AND actually viable (H.264) — a non-H.264 "webrtc" camera
// still needs HLS because browsers can't play it over WebRTC, and "auto"/"hls"
// always keep HLS (fallback / HLS-only). This is what lets a "webrtc" camera stop
// writing .ts segments entirely.
func ShouldRunHLS(videoCodec, transport string) bool {
	return !(transport == "webrtc" && videoCodec == "h264")
}
