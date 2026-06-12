package deviceinfo

import (
	"context"
	"testing"
)

type stubCollector struct {
	name    string
	detects bool
	values  map[string]string
}

func (s stubCollector) Name() string                        { return s.name }
func (s stubCollector) Detect(context.Context, Target) bool { return s.detects }
func (s stubCollector) Collect(context.Context, Target) (map[string]string, error) {
	return s.values, nil
}

type stubProber struct{ values map[string]string }

func (p stubProber) ProbeStream(context.Context, string) map[string]string { return p.values }

func TestCollectPicksDetectedCollectorAndMergesProbe(t *testing.T) {
	dahua := stubCollector{name: "dahua", detects: true, values: map[string]string{
		"collector":       "dahua",
		"model":           "iM5",
		"stream.main.gop": "40", // codec/res absent → filled from probe
	}}
	other := stubCollector{name: "never", detects: false}
	prober := stubProber{values: map[string]string{
		"stream.main.codec": "h264",
		"stream.main.width": "1920",
		"stream.main.gop":   "999", // must NOT override collector's value
	}}

	got := Collect(context.Background(), Target{RTSPURL: "rtsp://x"}, []Collector{other, dahua}, prober)

	if got["collector"] != "dahua" || got["model"] != "iM5" {
		t.Fatalf("wrong collector chosen: %v", got)
	}
	if got["stream.main.codec"] != "h264" || got["stream.main.width"] != "1920" {
		t.Errorf("probe keys not merged: %v", got)
	}
	if got["stream.main.gop"] != "40" {
		t.Errorf("probe overrode collector value: gop = %q, want 40", got["stream.main.gop"])
	}
}

func TestCollectFallsBackToGenericProbe(t *testing.T) {
	none := stubCollector{name: "dahua", detects: false}
	prober := stubProber{values: map[string]string{"stream.main.codec": "h264"}}

	got := Collect(context.Background(), Target{RTSPURL: "rtsp://x"}, []Collector{none}, prober)

	if got["collector"] != "generic" {
		t.Errorf("collector = %q, want generic", got["collector"])
	}
	if got["stream.main.codec"] != "h264" {
		t.Errorf("generic probe not captured: %v", got)
	}
}
