package discovery

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// StreamURI represents an available RTSP stream on an ONVIF camera.
type StreamURI struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// GetStreamURIs fetches available RTSP stream URIs from an ONVIF camera by
// calling GetCapabilities → GetProfiles → GetStreamUri.
func GetStreamURIs(ctx context.Context, xaddr, user, pass string) ([]StreamURI, error) {
	mediaURL := mediaEndpoint(ctx, xaddr, user, pass)

	profiles, err := getProfiles(ctx, mediaURL, user, pass)
	if err != nil {
		return nil, err
	}

	seen := map[string]bool{}
	var streams []StreamURI
	for _, p := range profiles {
		uri, err := getStreamURI(ctx, mediaURL, p.Token, user, pass)
		if err != nil || seen[uri] {
			continue
		}
		seen[uri] = true
		streams = append(streams, StreamURI{Name: p.Name, URL: uri})
	}
	return streams, nil
}

// mediaEndpoint discovers the media service URL via GetCapabilities,
// falling back to a URL derived from the device service XAddr.
func mediaEndpoint(ctx context.Context, xaddr, user, pass string) string {
	body, err := soapCall(ctx, xaddr, user, pass,
		`<GetCapabilities xmlns="http://www.onvif.org/ver10/device/wsdl"><Category>Media</Category></GetCapabilities>`)
	if err != nil {
		return deriveMediaURL(xaddr)
	}
	var resp struct {
		XAddr string `xml:"Body>GetCapabilitiesResponse>Capabilities>Media>XAddr"`
	}
	if xml.Unmarshal(body, &resp) != nil || resp.XAddr == "" {
		return deriveMediaURL(xaddr)
	}
	return resp.XAddr
}

type onvifProfile struct {
	Token string `xml:"token,attr"`
	Name  string `xml:"Name"`
}

func getProfiles(ctx context.Context, mediaURL, user, pass string) ([]onvifProfile, error) {
	body, err := soapCall(ctx, mediaURL, user, pass,
		`<GetProfiles xmlns="http://www.onvif.org/ver10/media/wsdl"/>`)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Profiles []onvifProfile `xml:"Body>GetProfilesResponse>Profiles"`
	}
	if err := xml.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse profiles: %w", err)
	}
	return resp.Profiles, nil
}

func getStreamURI(ctx context.Context, mediaURL, token, user, pass string) (string, error) {
	reqBody := fmt.Sprintf(`<GetStreamUri xmlns="http://www.onvif.org/ver10/media/wsdl">
  <StreamSetup>
    <Stream xmlns="http://www.onvif.org/ver10/schema">RTP-Unicast</Stream>
    <Transport xmlns="http://www.onvif.org/ver10/schema"><Protocol>RTSP</Protocol></Transport>
  </StreamSetup>
  <ProfileToken>%s</ProfileToken>
</GetStreamUri>`, token)

	body, err := soapCall(ctx, mediaURL, user, pass, reqBody)
	if err != nil {
		return "", err
	}
	var resp struct {
		URI string `xml:"Body>GetStreamUriResponse>MediaUri>Uri"`
	}
	if err := xml.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("parse stream uri: %w", err)
	}
	if resp.URI == "" {
		return "", fmt.Errorf("empty URI in response")
	}
	return resp.URI, nil
}

func soapCall(ctx context.Context, endpoint, user, pass, bodyContent string) ([]byte, error) {
	envelope := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope">
  <s:Header>%s</s:Header>
  <s:Body>%s</s:Body>
</s:Envelope>`, wsSecurityHeader(user, pass), bodyContent)

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBufferString(envelope))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/soap+xml; charset=utf-8")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("SOAP %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	return data, nil
}

func wsSecurityHeader(user, pass string) string {
	nonce := make([]byte, 16)
	_, _ = rand.Read(nonce)
	created := time.Now().UTC().Format(time.RFC3339)

	h := sha1.New()
	h.Write(nonce)
	h.Write([]byte(created))
	h.Write([]byte(pass))
	digest := base64.StdEncoding.EncodeToString(h.Sum(nil))
	nonceB64 := base64.StdEncoding.EncodeToString(nonce)

	return fmt.Sprintf(
		`<Security xmlns="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-secext-1.0.xsd">
  <UsernameToken>
    <Username>%s</Username>
    <Password Type="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-username-token-profile-1.0#PasswordDigest">%s</Password>
    <Nonce EncodingType="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-soap-message-security-1.0#Base64Binary">%s</Nonce>
    <Created xmlns="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-utility-1.0.xsd">%s</Created>
  </UsernameToken>
</Security>`,
		user, digest, nonceB64, created)
}

func deriveMediaURL(xaddr string) string {
	if i := strings.Index(xaddr, "/onvif/"); i >= 0 {
		return xaddr[:i] + "/onvif/media"
	}
	return strings.TrimRight(xaddr, "/") + "/onvif/media"
}
