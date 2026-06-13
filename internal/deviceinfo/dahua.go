package deviceinfo

import (
	"context"
	"strings"
)

// Dahua collects device info from Dahua/Intelbras cameras via their CGI API.
type Dahua struct {
	// dial builds the CGI client for a target; injectable for tests.
	dial func(Target) CGIClient
}

// NewDahua returns a Dahua collector using the real HTTP digest CGI client.
func NewDahua() *Dahua {
	return &Dahua{dial: newDigestCGI}
}

func (d *Dahua) Name() string { return "dahua" }

// Detect reports whether the target answers the Dahua device-type CGI.
func (d *Dahua) Detect(ctx context.Context, t Target) bool {
	body, err := d.dial(t).Get(ctx, "magicBox.cgi?action=getDeviceType")
	if err != nil {
		return false
	}
	return parseKV(body)["type"] != ""
}

// Collect queries the device's CGI endpoints and returns a flat map of
// namespaced keys (model, serial, firmware, mac, ntp.enabled, stream.main.*,
// ...) plus the full raw config dump under "raw.*". Endpoints that fail are
// skipped (best-effort); empty curated values are omitted.
func (d *Dahua) Collect(ctx context.Context, t Target) (map[string]string, error) {
	cgi := d.dial(t)
	out := map[string]string{"collector": d.Name()}

	get := func(query string) map[string]string {
		body, err := cgi.Get(ctx, query)
		if err != nil {
			return nil
		}
		return parseKV(body)
	}
	addRaw := func(kv map[string]string) {
		for k, v := range kv {
			out["raw."+k] = v
		}
	}

	out["model"] = get("magicBox.cgi?action=getDeviceType")["type"]
	out["serial"] = get("magicBox.cgi?action=getSerialNo")["sn"]
	// The doc documents "vendor=" but real Intelbras cameras answer "Vendor=".
	vendor := get("magicBox.cgi?action=getVendor")
	out["vendor"] = firstNonEmpty(vendor["vendor"], vendor["Vendor"])
	out["firmware"] = get("magicBox.cgi?action=getSoftwareVersion")["version"]
	out["hardware"] = get("magicBox.cgi?action=getHardwareVersion")["version"]

	ntp := get("configManager.cgi?action=getConfig&name=NTP")
	addRaw(ntp)
	out["ntp.enabled"] = ntp["table.NTP.Enable"]
	out["timezone"] = formatTimezone(ntp["table.NTP.TimeZone"])

	network := get("configManager.cgi?action=getConfig&name=Network")
	addRaw(network)
	out["mac"] = macFromNetwork(network)

	enc := get("configManager.cgi?action=getConfig&name=Encode")
	addRaw(enc)
	addStream(out, enc, "stream.main.", "table.Encode[0].MainFormat[0].Video.")
	addStream(out, enc, "stream.sub.", "table.Encode[0].ExtraFormat[0].Video.")

	for k, v := range out {
		if v == "" {
			delete(out, k)
		}
	}
	return out, nil
}

// addStream copies the curated video fields of one encoder stream from the
// Encode config (inPrefix) into namespaced keys (outPrefix), e.g.
// "stream.main.gop".
func addStream(out, enc map[string]string, outPrefix, inPrefix string) {
	fields := map[string]string{
		"codec":           "Compression",
		"width":           "Width",
		"height":          "Height",
		"fps":             "FPS",
		"gop":             "GOP",
		"bitrate":         "BitRate",
		"bitrate_control": "BitRateControl",
	}
	for outK, inK := range fields {
		if v := enc[inPrefix+inK]; v != "" {
			out[outPrefix+outK] = v
		}
	}
}

// dahuaTimezones maps the Dahua/Intelbras NTP.TimeZone index (a vendor-specific
// convention) to its UTC offset. From the Intelbras HTTP API table.
var dahuaTimezones = map[string]string{
	"0": "UTC+00:00", "1": "UTC+01:00", "2": "UTC+02:00", "3": "UTC+03:00",
	"4": "UTC+03:30", "5": "UTC+04:00", "6": "UTC+04:30", "7": "UTC+05:00",
	"8": "UTC+05:30", "9": "UTC+05:45", "10": "UTC+06:00", "11": "UTC+06:30",
	"12": "UTC+07:00", "13": "UTC+08:00", "14": "UTC+09:00", "15": "UTC+09:30",
	"16": "UTC+10:00", "17": "UTC+11:00", "18": "UTC+12:00", "19": "UTC+13:00",
	"20": "UTC-01:00", "21": "UTC-02:00", "22": "UTC-03:00", "23": "UTC-03:30",
	"24": "UTC-04:00", "25": "UTC-05:00", "26": "UTC-06:00", "27": "UTC-07:00",
	"28": "UTC-08:00", "29": "UTC-09:00", "30": "UTC-10:00", "31": "UTC-11:00",
	"32": "UTC-12:00", "33": "UTC-04:30", "34": "UTC+10:30", "35": "UTC+14:00",
	"36": "UTC-09:30", "37": "UTC+08:30", "38": "UTC+08:45", "39": "UTC+12:45",
}

// formatTimezone renders the timezone as "<index> / <offset>" (e.g.
// "22 / UTC-03:00"), keeping the raw index visible. Unknown indexes return the
// index alone.
func formatTimezone(idx string) string {
	if idx == "" {
		return ""
	}
	if offset, ok := dahuaTimezones[idx]; ok {
		return idx + " / " + offset
	}
	return idx
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// macFromNetwork extracts the MAC address from a Network config map, preferring
// eth0 but falling back to any interface's PhysicalAddress.
func macFromNetwork(kv map[string]string) string {
	if v := kv["table.Network.eth0.PhysicalAddress"]; v != "" {
		return v
	}
	for k, v := range kv {
		if strings.HasSuffix(k, ".PhysicalAddress") && v != "" {
			return v
		}
	}
	return ""
}
