// Package deviceinfo captures hardware/maintenance metadata about a camera
// (model, serial, firmware, network, encoder config, and — in the future —
// capabilities like zoom/focus and config URLs) right after it is registered.
//
// Captured data is a flat map[string]string of namespaced keys (e.g. "model",
// "ntp.enabled", "stream.main.gop", "capability.zoom", "url.config"), plus the
// full raw device dump under "raw.*". This EAV shape lets every camera model
// keep whatever it exposes without schema changes. Capture is extensible per
// camera family via the Collector interface; today only the Dahua/Intelbras CGI
// collector exists, with generic ffprobe stream keys merged on top.
package deviceinfo

import (
	"context"
	"strings"
)

// Target identifies a camera to probe.
type Target struct {
	Host     string
	RTSPURL  string
	Username string
	Password string
}

// CGIClient performs an authenticated GET against a camera's /cgi-bin endpoint,
// returning the response body. query is the part after /cgi-bin/, e.g.
// "magicBox.cgi?action=getDeviceType".
type CGIClient interface {
	Get(ctx context.Context, query string) (string, error)
}

// Collector captures device info for a family of cameras. Name reports the
// collector type (e.g. "dahua"); Detect cheaply reports whether it recognizes
// the device; Collect returns the captured key/value pairs.
type Collector interface {
	Name() string
	Detect(ctx context.Context, t Target) bool
	Collect(ctx context.Context, t Target) (map[string]string, error)
}

// Prober supplies generic stream keys (e.g. "stream.main.codec") from an RTSP
// probe, brand-agnostic, to fill gaps no specific collector covered.
type Prober interface {
	ProbeStream(ctx context.Context, rtspURL string) map[string]string
}

// Collect runs the first collector that detects the target, tags the result
// with a collector name, then merges generic stream-probe keys for any keys the
// collector did not provide. Best-effort: failures degrade gracefully.
func Collect(ctx context.Context, t Target, collectors []Collector, prober Prober) map[string]string {
	var out map[string]string
	for _, c := range collectors {
		if c.Detect(ctx, t) {
			out, _ = c.Collect(ctx, t)
			break
		}
	}
	if out == nil {
		out = map[string]string{}
	}
	if out["collector"] == "" {
		out["collector"] = "generic"
	}
	if prober != nil {
		for k, v := range prober.ProbeStream(ctx, t.RTSPURL) {
			if v != "" && out[k] == "" {
				out[k] = v
			}
		}
	}
	return out
}

// parseKV parses a CGI body in "key=value" form (one per line) into a map.
// Lines without '=' (e.g. "build:2021-06-01") are ignored.
func parseKV(body string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		i := strings.IndexByte(line, '=')
		if i <= 0 {
			continue
		}
		out[line[:i]] = strings.TrimSpace(line[i+1:])
	}
	return out
}
