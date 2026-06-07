package db_test

import (
	"context"
	"log/slog"
	"sync"
	"testing"

	"camera/internal/db"
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

func (h *captureHandler) findLevel(msg string, lvl slog.Level) (slog.Record, bool) {
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

// openMemDB opens an in-memory SQLite database with a capturing logger.
func openMemDB(t *testing.T, log *slog.Logger) *db.DB {
	t.Helper()
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	d.WithLogger(log)
	return d
}

// ── logging tests ────────────────────────────────────────────────────────────

func TestDBLogging_exec_debugLogsFastSuccess(t *testing.T) {
	log, h := newCapture(slog.LevelDebug)
	d := openMemDB(t, log)

	_, err := d.Exec(`CREATE TABLE IF NOT EXISTS t (id INTEGER PRIMARY KEY)`)
	if err != nil {
		t.Fatalf("exec: %v", err)
	}

	rec, ok := h.findLevel("db query", slog.LevelDebug)
	if !ok {
		t.Fatal("fast successful Exec should be logged at DEBUG")
	}
	if h.attrStr(rec, "sql") == "" {
		t.Error("debug db query log should include sql")
	}
	if h.attrStr(rec, "duration") == "" {
		t.Error("debug db query log should include duration")
	}
}

func TestDBLogging_query_debugLogsFastSuccess(t *testing.T) {
	log, h := newCapture(slog.LevelDebug)
	d := openMemDB(t, log)

	rows, err := d.Query(`SELECT 1`)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	rows.Close()

	if _, ok := h.findLevel("db query", slog.LevelDebug); !ok {
		t.Fatal("fast successful Query should be logged at DEBUG")
	}
}

func TestDBLogging_queryRow_debugLogsFastSuccess(t *testing.T) {
	log, h := newCapture(slog.LevelDebug)
	d := openMemDB(t, log)

	var n int
	_ = d.QueryRow(`SELECT 42`).Scan(&n)

	if _, ok := h.findLevel("db query", slog.LevelDebug); !ok {
		t.Fatal("fast successful QueryRow should be logged at DEBUG")
	}
}

func TestDBLogging_fastSuccess_notLoggedWhenDebugDisabled(t *testing.T) {
	log, h := newCapture(slog.LevelInfo)
	d := openMemDB(t, log)

	_, err := d.Exec(`CREATE TABLE IF NOT EXISTS t (id INTEGER PRIMARY KEY)`)
	if err != nil {
		t.Fatalf("exec: %v", err)
	}

	// With debug disabled, a fast successful query must stay silent.
	if _, ok := h.find("db query"); ok {
		t.Error("fast successful Exec must not be logged when debug is disabled")
	}
}

func TestDBLogging_execError_logsError(t *testing.T) {
	log, h := newCapture(slog.LevelInfo)
	d := openMemDB(t, log)

	// Force an error: INSERT into a nonexistent table.
	_, _ = d.Exec(`INSERT INTO nonexistent_table VALUES(1)`)

	_, ok := h.findLevel("db query", slog.LevelError)
	if !ok {
		t.Fatal("expected ERROR log for failed Exec")
	}
}

func TestDBLogging_noLogger_noPanic(t *testing.T) {
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer d.Close()

	// Without a logger, queries must still work normally.
	_, err = d.Exec(`SELECT 1`)
	if err != nil {
		t.Errorf("exec without logger: %v", err)
	}
}

func TestDBLogging_sqlTruncated_onError(t *testing.T) {
	log, h := newCapture(slog.LevelInfo)
	d := openMemDB(t, log)

	// Long SQL that fails — errors ARE logged so we can verify truncation.
	longSQL := `INSERT INTO nonexistent` + "                                                                                                                                    "
	_, _ = d.Exec(longSQL)

	rec, ok := h.findLevel("db query", slog.LevelError)
	if !ok {
		t.Fatal("expected ERROR log entry for failed query")
	}
	sql := h.attrStr(rec, "sql")
	if len(sql) > 120 {
		t.Errorf("sql should be truncated to 120 chars, got %d: %q", len(sql), sql)
	}
}
