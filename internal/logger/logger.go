package logger

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

type Options struct {
	Debug  bool
	Output string
	Path   string
}

func New(opts Options) (*slog.Logger, error) {
	switch opts.Output {
	case "stdout":
		return newStdoutLogger(opts.Debug), nil
	case "file":
		return newFileLogger(opts.Debug, opts.Path)
	default:
		return nil, fmt.Errorf("invalid log output %q: must be stdout or file", opts.Output)
	}
}

func newStdoutLogger(debug bool) *slog.Logger {
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
}

func newFileLogger(debug bool, path string) (*slog.Logger, error) {
	if err := os.MkdirAll(path, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	levels := []slog.Level{slog.LevelInfo, slog.LevelWarn, slog.LevelError}
	if debug {
		levels = append([]slog.Level{slog.LevelDebug}, levels...)
	}

	handlers := make([]slog.Handler, 0, len(levels))
	for _, level := range levels {
		f, err := os.OpenFile(filepath.Join(path, levelFilename(level)), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open %s: %w", levelFilename(level), err)
		}
		handlers = append(handlers, &levelHandler{
			level:   level,
			handler: slog.NewJSONHandler(f, &slog.HandlerOptions{Level: level}),
		})
	}

	return slog.New(&multiHandler{handlers: handlers}), nil
}

func levelFilename(level slog.Level) string {
	switch level {
	case slog.LevelDebug:
		return "debug.log"
	case slog.LevelInfo:
		return "info.log"
	case slog.LevelWarn:
		return "warn.log"
	case slog.LevelError:
		return "error.log"
	default:
		return "app.log"
	}
}

type levelHandler struct {
	level   slog.Level
	handler slog.Handler
}

func (h *levelHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level == h.level
}

func (h *levelHandler) Handle(ctx context.Context, r slog.Record) error {
	if r.Level != h.level {
		return nil
	}
	return h.handler.Handle(ctx, r)
}

func (h *levelHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &levelHandler{level: h.level, handler: h.handler.WithAttrs(attrs)}
}

func (h *levelHandler) WithGroup(name string) slog.Handler {
	return &levelHandler{level: h.level, handler: h.handler.WithGroup(name)}
}

type multiHandler struct {
	handlers []slog.Handler
}

func (h *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (h *multiHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, r.Level) {
			if err := handler.Handle(ctx, r); err != nil {
				return err
			}
		}
	}
	return nil
}

func (h *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		handlers[i] = handler.WithAttrs(attrs)
	}
	return &multiHandler{handlers: handlers}
}

func (h *multiHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		handlers[i] = handler.WithGroup(name)
	}
	return &multiHandler{handlers: handlers}
}
