package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"camera/internal/config"
	"camera/internal/db"
	"camera/internal/deviceinfo"
	"camera/internal/ffprobe"
)

// deviceInfoResponse is the API shape: the flat key/value snapshot plus when it
// was captured.
type deviceInfoResponse struct {
	CollectedAt time.Time         `json:"collected_at"`
	Values      map[string]string `json:"values"`
}

// WithDeviceInfoCollectors overrides the device-info collectors (tests inject
// fakes to avoid hitting real hardware). When unset, the real Dahua collector
// is used.
func (s *Server) WithDeviceInfoCollectors(collectors ...deviceinfo.Collector) *Server {
	s.deviceCollectors = collectors
	return s
}

func (s *Server) collectorsForDeviceInfo() []deviceinfo.Collector {
	if s.deviceCollectors != nil {
		return s.deviceCollectors
	}
	return []deviceinfo.Collector{deviceinfo.NewDahua()}
}

// captureDeviceInfo collects a camera's device-info snapshot and persists it.
func (s *Server) captureDeviceInfo(ctx context.Context, cam config.CameraConfig) (map[string]string, error) {
	values := deviceinfo.Collect(ctx, deviceInfoTarget(cam), s.collectorsForDeviceInfo(), ffprobeStreamProber{prober: s.prober})
	if s.db != nil {
		if err := db.SaveDeviceInfo(s.db, cam.ID, values); err != nil {
			return values, err
		}
	}
	return values, nil
}

// captureDeviceInfoAsync runs a best-effort capture in the background (used on
// camera create/update so the HTTP handler is not blocked).
func (s *Server) captureDeviceInfoAsync(cam config.CameraConfig) {
	go func() {
		if _, err := s.captureDeviceInfo(context.Background(), cam); err != nil {
			s.log.Warn("capture device info failed", "camera", cam.ID, "error", err)
		}
	}()
}

// CaptureMissingDeviceInfo captures and persists device info for every camera
// that has none yet. Meant to run once at startup (in a goroutine) so cameras
// registered before the feature existed get populated without manual action.
func (s *Server) CaptureMissingDeviceInfo(ctx context.Context) {
	if s.db == nil {
		return
	}
	cams, err := db.ListCameras(s.db)
	if err != nil {
		s.log.Warn("boot device-info capture: list cameras", "error", err)
		return
	}
	for _, cam := range cams {
		if _, _, ok, _ := db.GetDeviceInfo(s.db, cam.ID); ok {
			continue
		}
		if _, err := s.captureDeviceInfo(ctx, cam); err != nil {
			s.log.Warn("boot device-info capture failed", "camera", cam.ID, "error", err)
		}
	}
}

func deviceInfoTarget(cam config.CameraConfig) deviceinfo.Target {
	t := deviceinfo.Target{RTSPURL: cam.RTSPURL}
	if u, err := url.Parse(cam.RTSPURL); err == nil {
		t.Host = u.Hostname()
		if u.User != nil {
			t.Username = u.User.Username()
			t.Password, _ = u.User.Password()
		}
	}
	return t
}

// ffprobeStreamProber adapts ffprobe to deviceinfo.Prober, supplying generic
// stream.main.* keys for cameras whose collector data is partial.
type ffprobeStreamProber struct{ prober *ffprobe.Prober }

func (p ffprobeStreamProber) ProbeStream(ctx context.Context, rtspURL string) map[string]string {
	if p.prober == nil {
		return nil
	}
	raw, err := p.prober.Probe(ctx, rtspURL)
	if err != nil {
		return nil
	}
	si, err := ffprobe.Parse(raw)
	if err != nil {
		return nil
	}
	out := map[string]string{}
	if si.VideoCodec != "" {
		out["stream.main.codec"] = si.VideoCodec
	}
	if si.Width != 0 {
		out["stream.main.width"] = strconv.Itoa(si.Width)
	}
	if si.Height != 0 {
		out["stream.main.height"] = strconv.Itoa(si.Height)
	}
	return out
}

// handleDeviceInfo returns the most recently captured device-info snapshot.
func (s *Server) handleDeviceInfo(w http.ResponseWriter, r *http.Request) {
	if !s.requireDB(w) {
		return
	}
	values, collectedAt, ok, err := db.GetDeviceInfo(s.db, r.PathValue("id"))
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "no device info captured", http.StatusNotFound)
		return
	}
	writeJSON(w, deviceInfoResponse{CollectedAt: collectedAt, Values: values})
}

// handleRefreshDeviceInfo re-runs capture on demand and returns the fresh data.
func (s *Server) handleRefreshDeviceInfo(w http.ResponseWriter, r *http.Request) {
	if !s.requireDB(w) {
		return
	}
	cam, err := db.GetCamera(s.db, r.PathValue("id"))
	if err != nil {
		http.Error(w, "camera not found", http.StatusNotFound)
		return
	}
	values, err := s.captureDeviceInfo(r.Context(), cam)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, deviceInfoResponse{CollectedAt: time.Now().UTC(), Values: values})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
