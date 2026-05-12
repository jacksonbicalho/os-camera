package storage

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Cleaner struct {
	storagePath          string
	withMotionMinutes    int
	withoutMotionMinutes int
	defaultChunkDuration time.Duration
	chunkDurationsByCam  map[string]time.Duration
	maxSizeGB            float64
	warnPercent          float64
	log                  *slog.Logger
}

func New(storagePath string, withMotionMinutes, withoutMotionMinutes int, defaultChunkDuration time.Duration, chunkDurationsByCam map[string]time.Duration, maxSizeGB float64, warnPercent float64, log *slog.Logger) *Cleaner {
	return &Cleaner{
		storagePath:          storagePath,
		withMotionMinutes:    withMotionMinutes,
		withoutMotionMinutes: withoutMotionMinutes,
		defaultChunkDuration: defaultChunkDuration,
		chunkDurationsByCam:  chunkDurationsByCam,
		maxSizeGB:            maxSizeGB,
		warnPercent:          warnPercent,
		log:                  log,
	}
}

func cameraIDFromPath(path string) string {
	parts := strings.Split(filepath.ToSlash(path), "/")
	if len(parts) < 5 {
		return ""
	}
	return parts[len(parts)-5]
}

func (c *Cleaner) chunkDurationForPath(path string) time.Duration {
	cameraID := cameraIDFromPath(path)
	if cameraID != "" {
		if d, ok := c.chunkDurationsByCam[cameraID]; ok && d > 0 {
			return d
		}
	}
	return c.defaultChunkDuration
}

// ChunkStartFromName parses the UTC start time from a filename like "20060102150405.mp4".
func ChunkStartFromName(filename string) (time.Time, error) {
	base := strings.TrimSuffix(filename, filepath.Ext(filename))
	t, err := time.ParseInLocation("20060102150405", base, time.UTC)
	if err != nil {
		return time.Time{}, fmt.Errorf("cannot parse chunk start from %q: %w", filename, err)
	}
	return t, nil
}

// HasMotionInRange reports whether motion.ndjson at ndjsonPath contains any event
// with timestamp in [start, end).
func HasMotionInRange(ndjsonPath string, start, end time.Time) bool {
	f, err := os.Open(ndjsonPath)
	if err != nil {
		return false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var ev struct {
			Time string `json:"time"`
		}
		if err := json.Unmarshal(line, &ev); err != nil {
			continue
		}
		t, err := time.Parse(time.RFC3339, ev.Time)
		if err != nil {
			continue
		}
		if !t.Before(start) && t.Before(end) {
			return true
		}
	}
	return false
}

// RemoveEventsInRange remove do motion.ndjson as entradas cujo time cai em
// [start, end) e apaga os _motion.jpg referenciados. Apaga o próprio arquivo
// se ficar vazio. É um no-op se o arquivo não existir.
func RemoveEventsInRange(ndjsonPath string, start, end time.Time) error {
	data, err := os.ReadFile(ndjsonPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	dir := filepath.Dir(ndjsonPath)
	var kept [][]byte
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var ev struct {
			Time  string `json:"time"`
			Frame string `json:"frame"`
		}
		if err := json.Unmarshal(line, &ev); err != nil {
			kept = append(kept, append([]byte{}, line...))
			continue
		}
		t, err := time.Parse(time.RFC3339, ev.Time)
		if err != nil {
			kept = append(kept, append([]byte{}, line...))
			continue
		}
		if !t.Before(start) && t.Before(end) {
			if ev.Frame != "" {
				os.Remove(filepath.Join(dir, ev.Frame))
			}
		} else {
			kept = append(kept, append([]byte{}, line...))
		}
	}

	if len(kept) == 0 {
		return os.Remove(ndjsonPath)
	}
	out := make([]byte, 0)
	for _, line := range kept {
		out = append(out, line...)
		out = append(out, '\n')
	}
	return os.WriteFile(ndjsonPath, out, 0o644)
}

func (c *Cleaner) Clean() {
	if c.withMotionMinutes == 0 && c.withoutMotionMinutes == 0 {
		return
	}
	now := time.Now().UTC()
	filepath.WalkDir(c.storagePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".mp4" {
			return nil
		}
		chunkStart, err := ChunkStartFromName(filepath.Base(path))
		if err != nil {
			return nil
		}
		chunkDuration := c.chunkDurationForPath(path)
		chunkEnd := chunkStart.Add(chunkDuration)
		ndjsonPath := filepath.Join(filepath.Dir(path), "motion.ndjson")
		hasMotion := HasMotionInRange(ndjsonPath, chunkStart, chunkEnd)

		var retentionMinutes int
		if hasMotion {
			if c.withMotionMinutes == 0 {
				return nil
			}
			retentionMinutes = c.withMotionMinutes
		} else {
			if c.withoutMotionMinutes == 0 {
				return nil
			}
			retentionMinutes = c.withoutMotionMinutes
		}

		cutoff := now.Add(-time.Duration(retentionMinutes) * time.Minute)
		if chunkEnd.Before(cutoff) {
			c.log.Debug("deleting old recording", "path", path, "camera_id", cameraIDFromPath(path), "chunk_start", chunkStart, "chunk_duration", chunkDuration, "has_motion", hasMotion)
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
