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
	"sort"
	"strings"
	"time"

	"camera/internal/db"
)

type Cleaner struct {
	storagePath          string
	withMotionMinutes    int
	withoutMotionMinutes int
	chunkDuration        time.Duration
	maxSizeGB            float64
	warnPercent          float64
	db                   *db.DB
	log                  *slog.Logger
}

func New(storagePath string, withMotionMinutes, withoutMotionMinutes int, chunkDuration time.Duration, maxSizeGB float64, warnPercent float64, database *db.DB, log *slog.Logger) *Cleaner {
	return &Cleaner{
		storagePath:          storagePath,
		withMotionMinutes:    withMotionMinutes,
		withoutMotionMinutes: withoutMotionMinutes,
		chunkDuration:        chunkDuration,
		maxSizeGB:            maxSizeGB,
		warnPercent:          warnPercent,
		db:                   database,
		log:                  log,
	}
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

func (c *Cleaner) syncRecordings() {
	// Collect MP4 files grouped by directory so we can determine each chunk's
	// real end time from the next file's start time.
	byDir := map[string][]string{}
	filepath.WalkDir(c.storagePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".mp4" {
			return nil
		}
		dir := filepath.Dir(path)
		byDir[dir] = append(byDir[dir], filepath.Base(path))
		return nil
	})

	for dir, files := range byDir {
		sort.Strings(files)

		for i, name := range files {
			chunkStart, err := ChunkStartFromName(name)
			if err != nil {
				continue
			}

			// Use next file's start as chunkEnd; fall back to configured duration
			// for the last file in the directory.
			var chunkEnd time.Time
			if i+1 < len(files) {
				next, err := ChunkStartFromName(files[i+1])
				if err == nil {
					chunkEnd = next
				}
			}

			fullPath := filepath.Join(dir, name)
			info, err := os.Stat(fullPath)
			if err != nil {
				continue
			}

			// Extract cameraID: first path component relative to storagePath.
			rel, err := filepath.Rel(c.storagePath, fullPath)
			if err != nil {
				continue
			}
			parts := strings.SplitN(rel, string(filepath.Separator), 2)
			if len(parts) == 0 {
				continue
			}
			cameraID := parts[0]

			rec := db.Recording{
				CameraID:  cameraID,
				StartedAt: chunkStart,
				EndedAt:   chunkEnd,
				Path:      fullPath,
				SizeBytes: info.Size(),
				HasMotion: false,
			}
			if err := db.InsertRecording(c.db, rec); err != nil {
				c.log.Warn("failed to insert recording", "path", fullPath, "err", err)
			}
		}
	}

	// Batch-update has_motion for recordings that have overlapping motion events.
	_, err := c.db.Exec(`
		UPDATE recordings SET has_motion=1
		WHERE has_motion=0
		AND EXISTS (
			SELECT 1 FROM motion_events me
			WHERE me.camera_id = recordings.camera_id
			AND me.occurred_at >= recordings.started_at
			AND (recordings.ended_at IS NULL OR me.occurred_at < recordings.ended_at)
		)`)
	if err != nil {
		c.log.Warn("failed to update has_motion from motion_events", "err", err)
	}
}

func (c *Cleaner) cleanFromDB() {
	if c.withMotionMinutes == 0 && c.withoutMotionMinutes == 0 {
		return
	}
	now := time.Now().UTC()

	type row struct {
		path      string
		hasMotion bool
	}
	var toDelete []row

	if c.withoutMotionMinutes > 0 {
		cutoff := now.Add(-time.Duration(c.withoutMotionMinutes) * time.Minute).Format(time.RFC3339)
		rows, err := c.db.Query(`SELECT path FROM recordings WHERE has_motion=0 AND started_at < ?`, cutoff)
		if err != nil {
			c.log.Warn("failed to query without-motion recordings", "err", err)
		} else {
			defer rows.Close()
			for rows.Next() {
				var path string
				if err := rows.Scan(&path); err != nil {
					c.log.Warn("failed to scan recording path", "err", err)
					continue
				}
				toDelete = append(toDelete, row{path: path, hasMotion: false})
			}
		}
	}

	if c.withMotionMinutes > 0 {
		cutoff := now.Add(-time.Duration(c.withMotionMinutes) * time.Minute).Format(time.RFC3339)
		rows, err := c.db.Query(`SELECT path FROM recordings WHERE has_motion=1 AND started_at < ?`, cutoff)
		if err != nil {
			c.log.Warn("failed to query with-motion recordings", "err", err)
		} else {
			defer rows.Close()
			for rows.Next() {
				var path string
				if err := rows.Scan(&path); err != nil {
					c.log.Warn("failed to scan recording path", "err", err)
					continue
				}
				toDelete = append(toDelete, row{path: path, hasMotion: true})
			}
		}
	}

	for _, r := range toDelete {
		c.log.Debug("deleting old recording", "path", r.path, "has_motion", r.hasMotion)
		if err := os.Remove(r.path); err != nil && !os.IsNotExist(err) {
			c.log.Warn("failed to delete recording", "path", r.path, "err", err)
		}
		if err := db.DeleteRecording(c.db, r.path); err != nil {
			c.log.Warn("failed to delete recording from db", "path", r.path, "err", err)
		}
	}
}

func (c *Cleaner) cleanFromFS() {
	if c.withMotionMinutes == 0 && c.withoutMotionMinutes == 0 {
		return
	}
	now := time.Now().UTC()

	// Collect MP4 files grouped by directory so we can determine each chunk's
	// real end time from the next file's start time.
	byDir := map[string][]string{}
	filepath.WalkDir(c.storagePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".mp4" {
			return nil
		}
		dir := filepath.Dir(path)
		byDir[dir] = append(byDir[dir], filepath.Base(path))
		return nil
	})

	for dir, files := range byDir {
		sort.Strings(files)
		ndjsonPath := filepath.Join(dir, "motion.ndjson")

		for i, name := range files {
			chunkStart, err := ChunkStartFromName(name)
			if err != nil {
				continue
			}

			// Use next file's start as chunkEnd; fall back to configured duration
			// for the last file in the directory.
			var chunkEnd time.Time
			if i+1 < len(files) {
				next, err := ChunkStartFromName(files[i+1])
				if err == nil {
					chunkEnd = next
				}
			}
			if chunkEnd.IsZero() {
				chunkEnd = chunkStart.Add(c.chunkDuration)
			}

			hasMotion := HasMotionInRange(ndjsonPath, chunkStart, chunkEnd)

			var retentionMinutes int
			if hasMotion {
				if c.withMotionMinutes == 0 {
					continue
				}
				retentionMinutes = c.withMotionMinutes
			} else {
				if c.withoutMotionMinutes == 0 {
					continue
				}
				retentionMinutes = c.withoutMotionMinutes
			}

			cutoff := now.Add(-time.Duration(retentionMinutes) * time.Minute)
			if chunkStart.Before(cutoff) {
				path := filepath.Join(dir, name)
				c.log.Debug("deleting old recording", "path", path, "has_motion", hasMotion)
				if err := os.Remove(path); err != nil {
					c.log.Warn("failed to delete recording", "path", path, "err", err)
				}
			}
		}
	}
}

func (c *Cleaner) Clean() {
	if c.db != nil {
		c.syncRecordings()
		c.cleanFromDB()
	} else {
		c.cleanFromFS()
	}
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
