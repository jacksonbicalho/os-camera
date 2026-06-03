package server

import (
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// loggingResponseWriter wraps http.ResponseWriter to capture the status code.
type loggingResponseWriter struct {
	http.ResponseWriter
	code int
}

func (lw *loggingResponseWriter) WriteHeader(code int) {
	lw.code = code
	lw.ResponseWriter.WriteHeader(code)
}

// Flush forwards to the underlying writer so SSE and streaming handlers work.
func (lw *loggingResponseWriter) Flush() {
	if f, ok := lw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (s *Server) logRequest(r *http.Request, status int, dur time.Duration) {
	isAPI := strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/stream/")
	isDebug := s.log.Enabled(r.Context(), slog.LevelDebug)

	// Skip successful static-file responses in non-debug mode to avoid noise.
	if !isAPI && !isDebug && status < 400 {
		return
	}

	attrs := []any{
		"method", r.Method,
		"path", r.URL.Path,
		"status", status,
		"duration", dur.Round(time.Microsecond).String(),
	}

	if isDebug {
		attrs = append(attrs, "ip", r.RemoteAddr)
		if ua := r.Header.Get("User-Agent"); ua != "" {
			attrs = append(attrs, "ua", ua)
		}
		if q := redactQuery(r.URL.RawQuery); q != "" {
			attrs = append(attrs, "query", q)
		}
	}

	switch {
	case status >= 500:
		s.log.Error("http request", attrs...)
	case status >= 400:
		s.log.Warn("http request", attrs...)
	default:
		s.log.Info("http request", attrs...)
	}
}

// redactQuery returns the query string with the token parameter replaced by ***.
func redactQuery(raw string) string {
	if raw == "" {
		return ""
	}
	parts := strings.Split(raw, "&")
	for i, p := range parts {
		if strings.HasPrefix(p, "token=") || p == "token" {
			parts[i] = "token=***"
		}
	}
	return strings.Join(parts, "&")
}
