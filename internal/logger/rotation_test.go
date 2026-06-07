package logger

import "testing"

// newRotatingWriter is the only logic we own around lumberjack: mapping our
// resolved Options onto the writer's fields. Rotation/compression/retention
// behavior belongs to lumberjack and is not tested here.
func TestNewRotatingWriterMapsOptions(t *testing.T) {
	opts := Options{
		MaxSizeMB:  25,
		MaxAgeDays: 7,
		MaxBackups: 3,
		Compress:   true,
	}

	w := newRotatingWriter("/var/log/camera/info.log", opts)

	if w.Filename != "/var/log/camera/info.log" {
		t.Errorf("expected filename to be passed through, got %q", w.Filename)
	}
	if w.MaxSize != 25 {
		t.Errorf("expected MaxSize 25, got %d", w.MaxSize)
	}
	if w.MaxAge != 7 {
		t.Errorf("expected MaxAge 7, got %d", w.MaxAge)
	}
	if w.MaxBackups != 3 {
		t.Errorf("expected MaxBackups 3, got %d", w.MaxBackups)
	}
	if !w.Compress {
		t.Error("expected Compress true")
	}
}

func TestNewRotatingWriterPassesZeroValues(t *testing.T) {
	// 0 is meaningful to lumberjack (unlimited age/backups); the mapping must
	// pass it through verbatim rather than substituting a default.
	w := newRotatingWriter("/tmp/info.log", Options{MaxAgeDays: 0, MaxBackups: 0, Compress: false})

	if w.MaxAge != 0 {
		t.Errorf("expected MaxAge 0 passed through, got %d", w.MaxAge)
	}
	if w.MaxBackups != 0 {
		t.Errorf("expected MaxBackups 0 passed through, got %d", w.MaxBackups)
	}
	if w.Compress {
		t.Error("expected Compress false")
	}
}
