package discovery

import (
	"testing"
)

func TestParseProbeMatch_ExtractsXAddrsAndScopes(t *testing.T) {
	xml := `<?xml version="1.0" encoding="UTF-8"?>
<Envelope xmlns="http://www.w3.org/2003/05/soap-envelope">
  <Body>
    <ProbeMatches xmlns="http://docs.oasis-open.org/ws-dd/ns/discovery/2009/01">
      <ProbeMatch>
        <XAddrs>http://192.168.1.100/onvif/device_service</XAddrs>
        <Scopes>onvif://www.onvif.org/hardware/DS-2CD2143G2-I onvif://www.onvif.org/name/Hikvision</Scopes>
      </ProbeMatch>
    </ProbeMatches>
  </Body>
</Envelope>`

	results, err := parseProbeMatches([]byte(xml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.IP != "192.168.1.100" {
		t.Errorf("IP = %q, want 192.168.1.100", r.IP)
	}
	if r.Port != 80 {
		t.Errorf("Port = %d, want 80", r.Port)
	}
	if !r.ONVIF {
		t.Error("expected ONVIF=true")
	}
	if len(r.Services) == 0 {
		t.Error("expected at least one service URL")
	}
}

func TestParseProbeMatch_MultipleAddrs(t *testing.T) {
	xml := `<Envelope xmlns="http://www.w3.org/2003/05/soap-envelope">
  <Body>
    <ProbeMatches xmlns="http://docs.oasis-open.org/ws-dd/ns/discovery/2009/01">
      <ProbeMatch>
        <XAddrs>http://192.168.1.10/onvif/device http://192.168.1.10:8080/onvif/device</XAddrs>
        <Scopes>onvif://www.onvif.org/name/Dahua</Scopes>
      </ProbeMatch>
      <ProbeMatch>
        <XAddrs>http://192.168.1.20/onvif/device_service</XAddrs>
        <Scopes></Scopes>
      </ProbeMatch>
    </ProbeMatches>
  </Body>
</Envelope>`

	results, err := parseProbeMatches([]byte(xml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].IP != "192.168.1.10" {
		t.Errorf("first IP = %q, want 192.168.1.10", results[0].IP)
	}
	if results[1].IP != "192.168.1.20" {
		t.Errorf("second IP = %q, want 192.168.1.20", results[1].IP)
	}
}

func TestParseProbeMatch_NameFromScopes(t *testing.T) {
	xml := `<Envelope xmlns="http://www.w3.org/2003/05/soap-envelope">
  <Body>
    <ProbeMatches xmlns="http://docs.oasis-open.org/ws-dd/ns/discovery/2009/01">
      <ProbeMatch>
        <XAddrs>http://10.0.0.5/onvif/device_service</XAddrs>
        <Scopes>onvif://www.onvif.org/name/Reolink onvif://www.onvif.org/hardware/RLC-810A</Scopes>
      </ProbeMatch>
    </ProbeMatches>
  </Body>
</Envelope>`

	results, err := parseProbeMatches([]byte(xml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results[0].Name == "" {
		t.Error("expected non-empty Name parsed from scopes")
	}
}

func TestParseProbeMatch_EmptyBody(t *testing.T) {
	results, err := parseProbeMatches([]byte(`<Envelope xmlns="http://www.w3.org/2003/05/soap-envelope"><Body></Body></Envelope>`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestLocalSubnet_ReturnsSlashNotation(t *testing.T) {
	subnet, err := localSubnet()
	if err != nil {
		t.Skipf("no non-loopback interface: %v", err)
	}
	// Should be something like "192.168.1.0/24"
	if subnet == "" {
		t.Error("expected non-empty subnet")
	}
}
