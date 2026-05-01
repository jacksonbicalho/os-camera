package logger

import (
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
	level := slog.LevelInfo
	if opts.Debug {
		level = slog.LevelDebug
	}

	var w *os.File
	switch opts.Output {
	case "stdout":
		w = os.Stdout
	case "file":
		f, err := os.OpenFile(filepath.Join(opts.Path, "camera.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}
		w = f
	default:
		return nil, fmt.Errorf("invalid log output %q: must be stdout or file", opts.Output)
	}

	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level})), nil
}
