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
	out["timezone"] = ntp["table.NTP.TimeZone"]

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
