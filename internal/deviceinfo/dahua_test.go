package deviceinfo

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// fakeCGI returns canned CGI bodies, matched by substring of the query.
type fakeCGI struct{ resp map[string]string }

func (f fakeCGI) Get(_ context.Context, query string) (string, error) {
	for k, v := range f.resp {
		if strings.Contains(query, k) {
			return v, nil
		}
	}
	return "", fmt.Errorf("fakeCGI: no canned response for %q", query)
}

func dahuaWith(resp map[string]string) *Dahua {
	return &Dahua{dial: func(Target) CGIClient { return fakeCGI{resp: resp} }}
}

func TestDahuaCollectIdentityAndNTP(t *testing.T) {
	d := dahuaWith(map[string]string{
		"action=getDeviceType":      "type=iM5",
		"action=getSerialNo":        "sn=6L08ABCDEF",
		"action=getVendor":          "Vendor=IntelBras", // real cameras use a capital V
		"action=getSoftwareVersion": "version=2.800.0000000.1.R\nbuild:2021-06-01",
		"action=getHardwareVersion": "version=1.00",
		"name=NTP":                  "table.NTP.Enable=true\ntable.NTP.TimeZone=22",
		"name=Network":              "table.Network.eth0.PhysicalAddress=3c:ef:8c:11:22:33",
	})

	got, err := d.Collect(context.Background(), Target{Host: "192.168.1.29"})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}

	want := map[string]string{
		"collector":   "dahua",
		"vendor":      "IntelBras",
		"model":       "iM5",
		"serial":      "6L08ABCDEF",
		"firmware":    "2.800.0000000.1.R",
		"hardware":    "1.00",
		"ntp.enabled": "true",
		"timezone":    "22",
		"mac":         "3c:ef:8c:11:22:33",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("got[%q] = %q, want %q", k, got[k], v)
		}
	}
	// The full raw dump is preserved under raw.*
	if got["raw.table.NTP.Enable"] != "true" {
		t.Errorf("raw dump not preserved: %q", got["raw.table.NTP.Enable"])
	}
}

func TestDahuaName(t *testing.T) {
	if got := (&Dahua{}).Name(); got != "dahua" {
		t.Errorf("Name() = %q, want %q", got, "dahua")
	}
}

func TestDahuaCollectStreams(t *testing.T) {
	enc := strings.Join([]string{
		"table.Encode[0].MainFormat[0].Video.Compression=H.264",
		"table.Encode[0].MainFormat[0].Video.Width=1920",
		"table.Encode[0].MainFormat[0].Video.Height=1080",
		"table.Encode[0].MainFormat[0].Video.FPS=20",
		"table.Encode[0].MainFormat[0].Video.GOP=40",
		"table.Encode[0].MainFormat[0].Video.BitRate=1536",
		"table.Encode[0].MainFormat[0].Video.BitRateControl=VBR",
		"table.Encode[0].ExtraFormat[0].Video.Compression=H.264",
		"table.Encode[0].ExtraFormat[0].Video.Width=640",
		"table.Encode[0].ExtraFormat[0].Video.Height=480",
		"table.Encode[0].ExtraFormat[0].Video.FPS=15",
		"table.Encode[0].ExtraFormat[0].Video.GOP=60",
	}, "\n")

	d := dahuaWith(map[string]string{
		"action=getDeviceType": "type=iM5",
		"name=Encode":          enc,
	})

	got, err := d.Collect(context.Background(), Target{Host: "192.168.1.29"})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}

	want := map[string]string{
		"stream.main.codec":   "H.264",
		"stream.main.width":   "1920",
		"stream.main.height":  "1080",
		"stream.main.fps":     "20",
		"stream.main.gop":     "40",
		"stream.main.bitrate": "1536",
		"stream.sub.width":    "640",
		"stream.sub.gop":      "60",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("got[%q] = %q, want %q", k, got[k], v)
		}
	}
}
