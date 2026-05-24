package discovery

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func mockONVIFServer(t *testing.T) (srv *httptest.Server, srvURL *string) {
	t.Helper()
	u := new(string)
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		body := string(data)

		switch {
		case strings.Contains(body, "GetCapabilities"):
			fmt.Fprintf(w, `<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"><s:Body>
			  <GetCapabilitiesResponse xmlns="http://www.onvif.org/ver10/device/wsdl">
			    <Capabilities><Media><XAddr>%s/onvif/media</XAddr></Media></Capabilities>
			  </GetCapabilitiesResponse></s:Body></s:Envelope>`, *u)
		case strings.Contains(body, "GetProfiles"):
			w.Write([]byte(`<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"><s:Body>
			  <GetProfilesResponse xmlns="http://www.onvif.org/ver10/media/wsdl">
			    <Profiles token="P1"><Name>MainStream</Name></Profiles>
			    <Profiles token="P2"><Name>SubStream</Name></Profiles>
			  </GetProfilesResponse></s:Body></s:Envelope>`))
		case strings.Contains(body, "GetStreamUri") && strings.Contains(body, "P2"):
			w.Write([]byte(`<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"><s:Body>
			  <GetStreamUriResponse xmlns="http://www.onvif.org/ver10/media/wsdl">
			    <MediaUri><Uri>rtsp://192.168.1.100:554/sub</Uri></MediaUri>
			  </GetStreamUriResponse></s:Body></s:Envelope>`))
		case strings.Contains(body, "GetStreamUri"):
			w.Write([]byte(`<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"><s:Body>
			  <GetStreamUriResponse xmlns="http://www.onvif.org/ver10/media/wsdl">
			    <MediaUri><Uri>rtsp://192.168.1.100:554/main</Uri></MediaUri>
			  </GetStreamUriResponse></s:Body></s:Envelope>`))
		default:
			http.Error(w, "unknown request", http.StatusBadRequest)
		}
	}))
	*u = srv.URL
	return srv, u
}

func TestGetStreamURIs_ReturnsBothStreams(t *testing.T) {
	srv, srvURL := mockONVIFServer(t)
	defer srv.Close()

	streams, err := GetStreamURIs(context.Background(), *srvURL+"/onvif/device_service", "admin", "pass")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(streams) != 2 {
		t.Fatalf("expected 2 streams, got %d: %+v", len(streams), streams)
	}
	if streams[0].Name != "MainStream" || streams[0].URL != "rtsp://192.168.1.100:554/main" {
		t.Errorf("stream[0] = %+v", streams[0])
	}
	if streams[1].Name != "SubStream" || streams[1].URL != "rtsp://192.168.1.100:554/sub" {
		t.Errorf("stream[1] = %+v", streams[1])
	}
}

func TestGetStreamURIs_AuthFailed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	}))
	defer srv.Close()

	_, err := GetStreamURIs(context.Background(), srv.URL+"/onvif/device_service", "wrong", "pass")
	if err == nil {
		t.Fatal("expected error for 401 response, got nil")
	}
}

func TestGetStreamURIs_DeduplicatesURLs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		body := string(data)
		switch {
		case strings.Contains(body, "GetCapabilities"):
			http.Error(w, "not supported", http.StatusInternalServerError)
		case strings.Contains(body, "GetProfiles"):
			w.Write([]byte(`<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"><s:Body>
			  <GetProfilesResponse xmlns="http://www.onvif.org/ver10/media/wsdl">
			    <Profiles token="P1"><Name>Main</Name></Profiles>
			    <Profiles token="P2"><Name>Also Main</Name></Profiles>
			  </GetProfilesResponse></s:Body></s:Envelope>`))
		case strings.Contains(body, "GetStreamUri"):
			// Both profiles return the same URL
			w.Write([]byte(`<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"><s:Body>
			  <GetStreamUriResponse xmlns="http://www.onvif.org/ver10/media/wsdl">
			    <MediaUri><Uri>rtsp://192.168.1.100:554/stream</Uri></MediaUri>
			  </GetStreamUriResponse></s:Body></s:Envelope>`))
		}
	}))
	defer srv.Close()

	streams, err := GetStreamURIs(context.Background(), srv.URL+"/onvif/device_service", "admin", "pass")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(streams) != 1 {
		t.Errorf("expected 1 deduplicated stream, got %d: %+v", len(streams), streams)
	}
}

func TestDeriveMediaURL(t *testing.T) {
	cases := []struct{ in, want string }{
		{"http://192.168.1.100:80/onvif/device_service", "http://192.168.1.100:80/onvif/media"},
		{"http://192.168.1.100/onvif/Media", "http://192.168.1.100/onvif/media"},
		{"http://192.168.1.100:8080/other", "http://192.168.1.100:8080/other/onvif/media"},
	}
	for _, c := range cases {
		got := deriveMediaURL(c.in)
		if got != c.want {
			t.Errorf("deriveMediaURL(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
