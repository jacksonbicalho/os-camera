package storage

import (
	"context"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

type Cleaner struct {
	storagePath      string
	retentionMinutes int
	maxSizeGB        float64
	warnPercent      float64
	log              *slog.Logger
}

func New(storagePath string, retentionMinutes int, maxSizeGB float64, warnPercent float64, log *slog.Logger) *Cleaner {
	return &Cleaner{
		storagePath:      storagePath,
		retentionMinutes: retentionMinutes,
		maxSizeGB:        maxSizeGB,
		warnPercent:      warnPercent,
		log:              log,
	}
}

func (c *Cleaner) Clean() {
	if c.retentionMinutes == 0 {
		return
	}
	cutoff := time.Now().Add(-time.Duration(c.retentionMinutes) * time.Minute)
	filepath.WalkDir(c.storagePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".mp4" {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if info.ModTime().Before(cutoff) {
			c.log.Debug("deleting old recording", "path", path)
			if err := os.Remove(path); err != nil {
				c.log.Warn("failed to delete recording", "path", path, "err", err)
			}
		}
		return nil
	})
}

func (c *Cleaner) CheckSize() {
	if c.maxSizeGB == 0 {
		return
	}
	var totalBytes int64
	filepath.WalkDir(c.storagePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".mp4" {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		totalBytes += info.Size()
		return nil
	})
	maxBytes := int64(c.maxSizeGB * 1024 * 1024 * 1024)
	warnBytes := int64(float64(maxBytes) * c.warnPercent / 100)
	if totalBytes >= warnBytes {
		usedGB := float64(totalBytes) / (1024 * 1024 * 1024)
		c.log.Warn("storage usage high",
			"used_gb", usedGB,
			"max_gb", c.maxSizeGB,
			"warn_percent", c.warnPercent,
		)
	}
}

func (c *Cleaner) Run(ctx context.Context, interval time.Duration) {
	c.Clean()
	c.CheckSize()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.Clean()
			c.CheckSize()
		}
	}
}
