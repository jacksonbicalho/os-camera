package server_test

import (
	"context"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"camera/internal/config"
	"camera/internal/server"
)

// captureHandler records slog.Records for assertion in tests.
type captureHandler struct {
	mu       sync.Mutex
	records  []slog.Record
	minLevel slog.Level
}

func (h *captureHandler) Enabled(_ context.Context, lvl slog.Level) bool {
	return lvl >= h.minLevel
}

func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, r.Clone())
	return nil
}

func (h *captureHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *captureHandler) WithGroup(string) slog.Handler      { return h }

func (h *captureHandler) find(msg string) (slog.Record, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, r := range h.records {
		if r.Message == msg {
			return r, true
		}
	}
	return slog.Record{}, false
}

func (h *captureHandler) findAtLevel(msg string, lvl slog.Level) (slog.Record, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, r := range h.records {
		if r.Message == msg && r.Level == lvl {
			return r, true
		}
	}
	return slog.Record{}, false
}

func (h *captureHandler) attrStr(r slog.Record, key string) string {
	var val string
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == key {
			val = a.Value.String()
			return false
		}
		return true
	})
	return val
}

func newCapture(minLevel slog.Level) (*slog.Logger, *captureHandler) {
	h := &captureHandler{minLevel: minLevel}
	return slog.New(h), h
}

// ── logging middleware ───────────────────────────────────────────────────────

func TestHTTPLogging_200APIRequest_logsInfo(t *testing.T) {
	log, h := newCapture(slog.LevelInfo)
	srv := server.NewServer(config.ServerConfig{}, "UTC", nil, log, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	srv.ServeHTTP(httptest.NewRecorder(), req)

	rec, ok := h.find("http request")
	if !ok {
		t.Fatal("expected 'http request' log entry")
	}
	if rec.Level != slog.LevelInfo {
		t.Errorf("expected INFO, got %s", rec.Level)
	}
	if h.attrStr(rec, "status") != "200" {
		t.Errorf("expected status=200, got %s", h.attrStr(rec, "status"))
	}
	if h.attrStr(rec, "method") != http.MethodGet {
		t.Errorf("expected method=GET, got %s", h.attrStr(rec, "method"))
	}
}

func TestHTTPLogging_401_logsWarn(t *testing.T) {
	log, h := newCapture(slog.LevelInfo)
	srv := server.NewServer(config.ServerConfig{}, "UTC", nil, log, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/cameras", nil) // no auth
	srv.ServeHTTP(httptest.NewRecorder(), req)

	rec, ok := h.find("http request")
	if !ok {
		t.Fatal("expected 'http request' log entry")
	}
	if rec.Level != slog.LevelWarn {
		t.Errorf("expected WARN, got %s", rec.Level)
	}
	if h.attrStr(rec, "status") != "401" {
		t.Errorf("expected status=401, got %s", h.attrStr(rec, "status"))
	}
}

func TestHTTPLogging_5xx_logsError(t *testing.T) {
	log, h := newCapture(slog.LevelInfo)
	srv := server.NewServer(config.ServerConfig{}, "UTC", nil, log, nil)

	// login with nil DB returns 503
	body := `{"username":"admin","password":"pass"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	srv.ServeHTTP(httptest.NewRecorder(), req)

	rec, ok := h.find("http request")
	if !ok {
		t.Fatal("expected 'http request' log entry")
	}
	if rec.Level != slog.LevelError {
		t.Errorf("expected ERROR, got %s", rec.Level)
	}
}

func minimalFrontend() fs.FS {
	return fstest.MapFS{
		"index.html": {Data: []byte("<html/>")},
	}
}

func TestHTTPLogging_staticFile_notLoggedAtInfo(t *testing.T) {
	log, h := newCapture(slog.LevelInfo)
	srv := server.NewServer(config.ServerConfig{}, "UTC", nil, log, minimalFrontend())

	req := httptest.NewRequest(http.MethodGet, "/index.html", nil)
	srv.ServeHTTP(httptest.NewRecorder(), req)

	if _, ok := h.findAtLevel("http request", slog.LevelInfo); ok {
		t.Error("static file 2xx should not produce an INFO log entry")
	}
}

func TestHTTPLogging_debug_logsStaticAndExtraFields(t *testing.T) {
	log, h := newCapture(slog.LevelDebug)
	srv := server.NewServer(config.ServerConfig{}, "UTC", nil, log, minimalFrontend())

	req := httptest.NewRequest(http.MethodGet, "/index.html", nil)
	req.Header.Set("User-Agent", "TestBrowser/1.0")
	srv.ServeHTTP(httptest.NewRecorder(), req)

	rec, ok := h.find("http request")
	if !ok {
		t.Fatal("in DEBUG mode static file should be logged")
	}
	if h.attrStr(rec, "ip") == "" {
		t.Error("expected ip field in DEBUG mode")
	}
	if h.attrStr(rec, "ua") != "TestBrowser/1.0" {
		t.Errorf("expected ua=TestBrowser/1.0, got %s", h.attrStr(rec, "ua"))
	}
}

func TestHTTPLogging_tokenQueryParam_redacted(t *testing.T) {
	log, h := newCapture(slog.LevelDebug)
	srv := server.NewServer(config.ServerConfig{}, "UTC", nil, log, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/cameras?token=supersecret&date=2026-06-02", nil)
	srv.ServeHTTP(httptest.NewRecorder(), req)

	rec, ok := h.find("http request")
	if !ok {
		t.Fatal("expected 'http request' log entry")
	}
	query := h.attrStr(rec, "query")
	if strings.Contains(query, "supersecret") {
		t.Errorf("token must be redacted in query log, got: %s", query)
	}
	if !strings.Contains(query, "***") {
		t.Errorf("expected *** placeholder for token in query, got: %s", query)
	}
	if !strings.Contains(query, "date") {
		t.Errorf("non-sensitive query params should be preserved, got: %s", query)
	}
}
