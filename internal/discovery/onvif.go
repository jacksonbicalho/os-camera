package discovery

import (
	"context"
	"encoding/xml"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	wsdMulticast = "239.255.255.250:3702"
	wsdTimeout   = 3 * time.Second
)

var probeTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"
            xmlns:wsd="http://docs.oasis-open.org/ws-dd/ns/discovery/2009/01"
            xmlns:dn="http://www.onvif.org/ver10/network/wsdl">
  <s:Header>
    <wsd:MessageID>uuid:%s</wsd:MessageID>
    <wsd:To>urn:docs-oasis-open-org:ws-dd:ns:discovery:2009:01</wsd:To>
    <wsd:Action>http://docs.oasis-open.org/ws-dd/ns/discovery/2009/01/Probe</wsd:Action>
  </s:Header>
  <s:Body>
    <wsd:Probe>
      <wsd:Types>dn:NetworkVideoTransmitter</wsd:Types>
    </wsd:Probe>
  </s:Body>
</s:Envelope>`

// discoverONVIF sends a WS-Discovery probe and returns responding cameras.
func discoverONVIF(ctx context.Context) []Result {
	probe := fmt.Sprintf(probeTemplate, uuid.New().String())

	conn, err := net.ListenUDP("udp4", &net.UDPAddr{Port: 0})
	if err != nil {
		return nil
	}
	defer conn.Close()

	deadline := time.Now().Add(wsdTimeout)
	_ = conn.SetDeadline(deadline)

	dest, err := net.ResolveUDPAddr("udp4", wsdMulticast)
	if err != nil {
		return nil
	}
	if _, err := conn.WriteToUDP([]byte(probe), dest); err != nil {
		return nil
	}

	seen := map[string]bool{}
	var results []Result
	buf := make([]byte, 65536)

	for {
		select {
		case <-ctx.Done():
			return results
		default:
		}
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			break
		}
		parsed, err := parseProbeMatches(buf[:n])
		if err != nil {
			continue
		}
		for _, r := range parsed {
			if seen[r.IP] {
				continue
			}
			seen[r.IP] = true
			results = append(results, r)
		}
	}
	return results
}

// XML structs for WS-Discovery ProbeMatches response.
type wsProbeMatches struct {
	Matches []wsProbeMatch `xml:"Body>ProbeMatches>ProbeMatch"`
}

type wsProbeMatch struct {
	XAddrs string `xml:"XAddrs"`
	Scopes string `xml:"Scopes"`
}

func parseProbeMatches(data []byte) ([]Result, error) {
	var envelope wsProbeMatches
	if err := xml.Unmarshal(data, &envelope); err != nil {
		return nil, err
	}
	var results []Result
	for _, m := range envelope.Matches {
		addrs := strings.Fields(m.XAddrs)
		if len(addrs) == 0 {
			continue
		}
		// Use first XAddr to extract IP and port.
		u, err := url.Parse(addrs[0])
		if err != nil {
			continue
		}
		host := u.Hostname()
		port := 80
		if p := u.Port(); p != "" {
			fmt.Sscanf(p, "%d", &port)
		}
		r := Result{
			IP:         host,
			Port:       port,
			ONVIF:      true,
			ONVIFXAddr: addrs[0],
			Services:   addrs,
			Name:       nameFromScopes(m.Scopes),
		}
		results = append(results, r)
	}
	return results, nil
}

// nameFromScopes extracts a human-readable name from ONVIF scope URIs.
// Prefers onvif://www.onvif.org/name/<value>, falls back to hardware.
func nameFromScopes(scopes string) string {
	var name, hardware string
	for _, scope := range strings.Fields(scopes) {
		u, err := url.Parse(scope)
		if err != nil {
			continue
		}
		parts := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
		if len(parts) < 2 {
			continue
		}
		key, val := parts[len(parts)-2], parts[len(parts)-1]
		switch key {
		case "name":
			name = val
		case "hardware":
			hardware = val
		}
	}
	if name != "" && hardware != "" {
		return name + " " + hardware
	}
	if name != "" {
		return name
	}
	return hardware
}
