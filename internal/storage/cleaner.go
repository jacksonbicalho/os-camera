package storage

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"camera/internal/analysis"
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
	forceCh              chan struct{}
	analyzer             analysis.Analyzer
}

func (c *Cleaner) WithAnalyzer(a analysis.Analyzer) *Cleaner {
	c.analyzer = a
	return c
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
		forceCh:              make(chan struct{}, 1),
	}
}

// ForceClean schedules an immediate clean without waiting for the next interval.
// Safe to call concurrently; excess signals are dropped.
func (c *Cleaner) ForceClean() {
	select {
	case c.forceCh <- struct{}{}:
	default:
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
			if !chunkEnd.IsZero() {
				if err := db.UpdateRecordingEndedAt(c.db, fullPath, chunkEnd); err != nil {
					c.log.Warn("failed to update recording ended_at", "path", fullPath, "err", err)
				}
			}
		}
	}

	// Batch-update has_motion for recordings whose time range overlaps with
	// [event.occurred_at - lead, event.occurred_at + trail], using per-camera
	// playback_lead_seconds and playback_trail_seconds from camera_motion.
	// strftime keeps RFC3339 format (T separator, Z suffix) so comparisons with
	// started_at/ended_at — also stored as RFC3339 — are lexicographically correct.
	_, err := c.db.Exec(`
		UPDATE recordings SET has_motion=1
		WHERE has_motion=0
		AND EXISTS (
			SELECT 1
			FROM motion_events me
			JOIN camera_motion cm ON cm.camera_id = me.camera_id
			WHERE me.camera_id = recordings.camera_id
			AND recordings.started_at < strftime('%Y-%m-%dT%H:%M:%SZ', me.occurred_at, '+' || cm.playback_trail_seconds || ' seconds')
			AND (
				recordings.ended_at IS NULL
				OR recordings.ended_at > strftime('%Y-%m-%dT%H:%M:%SZ', me.occurred_at, '-' || cm.playback_lead_seconds || ' seconds')
			)
		)`)
	if err != nil {
		c.log.Warn("failed to update has_motion from motion_events", "err", err)
	}
}

// effectiveRetentionMinutes returns the retention minutes, preferring DB
// overrides (storage.with_motion_minutes / storage.without_motion_minutes)
// over the values set at construction time.
func (c *Cleaner) effectiveRetentionMinutes() (withMotion, withoutMotion int) {
	withMotion = c.withMotionMinutes
	withoutMotion = c.withoutMotionMinutes
	if c.db == nil {
		return
	}
	all, err := db.GetAllConfig(c.db)
	if err != nil {
		return
	}
	if v, ok := all["storage.with_motion_minutes"]; ok {
		if n, err := strconv.Atoi(v); err == nil {
			withMotion = n
		}
	}
	if v, ok := all["storage.without_motion_minutes"]; ok {
		if n, err := strconv.Atoi(v); err == nil {
			withoutMotion = n
		}
	}
	return
}

func (c *Cleaner) loadDrives() (drives map[string]Drive, withMotionAction, withoutMotionAction string, withMotionDriveID, withoutMotionDriveID string) {
	drives = make(map[string]Drive)
	withMotionAction = "delete"
	withoutMotionAction = "delete"

	dbDrives, err := db.ListDrives(c.db)
	if err != nil {
		c.log.Warn("failed to load drives from db", "err", err)
		return
	}
	for _, dr := range dbDrives {
		if dr.Type == "s3" {
			drives[dr.ID] = NewS3Drive(dr)
		}
	}

	configs, err := db.ListRetentionConfigs(c.db)
	if err != nil {
		c.log.Warn("failed to load retention configs from db", "err", err)
		return
	}
	for _, rc := range configs {
		switch rc.Category {
		case "with_motion":
			withMotionAction = rc.Action
			withMotionDriveID = rc.DriveID
		case "without_motion":
			withoutMotionAction = rc.Action
			withoutMotionDriveID = rc.DriveID
		}
	}
	return
}

func (c *Cleaner) cleanFromDB() {
	withMotionMinutes, withoutMotionMinutes := c.effectiveRetentionMinutes()
	if withMotionMinutes == 0 && withoutMotionMinutes == 0 {
		return
	}
	now := time.Now().UTC()

	drives, withMotionAction, withoutMotionAction, withMotionDriveID, withoutMotionDriveID := c.loadDrives()

	type row struct {
		path      string
		hasMotion bool
		startedAt time.Time
		endedAt   time.Time
	}
	var toProcess []row

	scanRows := func(sqlRows *sql.Rows, hasMotion bool) {
		defer sqlRows.Close()
		for sqlRows.Next() {
			var path, startedAtStr, endedAtStr string
			if err := sqlRows.Scan(&path, &startedAtStr, &endedAtStr); err != nil {
				c.log.Warn("failed to scan recording row", "err", err)
				continue
			}
			startedAt, _ := time.Parse(time.RFC3339, startedAtStr)
			endedAt, _ := time.Parse(time.RFC3339, endedAtStr)
			toProcess = append(toProcess, row{path: path, hasMotion: hasMotion, startedAt: startedAt, endedAt: endedAt})
		}
	}

	if withoutMotionMinutes > 0 {
		cutoff := now.Add(-time.Duration(withoutMotionMinutes) * time.Minute).Format(time.RFC3339)
		rows, err := c.db.Query(`SELECT path, started_at, ended_at FROM recordings WHERE has_motion=0 AND ended_at IS NOT NULL AND started_at < ?`, cutoff)
		if err != nil {
			c.log.Warn("failed to query without-motion recordings", "err", err)
		} else {
			scanRows(rows, false)
		}
	}

	if withMotionMinutes > 0 {
		cutoff := now.Add(-time.Duration(withMotionMinutes) * time.Minute).Format(time.RFC3339)
		rows, err := c.db.Query(`SELECT path, started_at, ended_at FROM recordings WHERE has_motion=1 AND ended_at IS NOT NULL AND started_at < ?`, cutoff)
		if err != nil {
			c.log.Warn("failed to query with-motion recordings", "err", err)
		} else {
			scanRows(rows, true)
		}
	}

	for _, r := range toProcess {
		action := withoutMotionAction
		driveID := withoutMotionDriveID
		if r.hasMotion {
			action = withMotionAction
			driveID = withMotionDriveID
		}

		if action == "send_to_drive" {
			drive, ok := drives[driveID]
			if !ok {
				c.log.Warn("drive not found for retention action, skipping", "drive_id", driveID, "path", r.path)
				continue
			}
			if err := c.uploadAndPurge(drive, r.path, r.startedAt, r.endedAt); err != nil {
				c.log.Warn("failed to send recording to drive, skipping deletion", "path", r.path, "err", err)
				continue
			}
			continue
		}

		// Default: delete
		c.log.Debug("deleting old recording", "path", r.path, "has_motion", r.hasMotion)
		if err := os.Remove(r.path); err != nil && !os.IsNotExist(err) {
			c.log.Warn("failed to delete recording", "path", r.path, "err", err)
		}
		c.purgeMotionAssets(r.path, r.startedAt, r.endedAt)
		if err := db.DeleteRecording(c.db, r.path); err != nil {
			c.log.Warn("failed to delete recording from db", "path", r.path, "err", err)
		}
	}
}

// purgeMotionAssets deletes motion_events rows and their JPEG files for the
// given recording time range. JPEGs are resolved from frame_path in the DB
// (the legacy motion.ndjson path is also tried for backwards-compatibility).
func (c *Cleaner) purgeMotionAssets(path string, startedAt, endedAt time.Time) {
	if startedAt.IsZero() || endedAt.IsZero() {
		return
	}

	// Legacy: ndjson-backed installations (no DB or ndjson still present).
	ndjsonPath := filepath.Join(filepath.Dir(path), "motion.ndjson")
	if err := RemoveEventsInRange(ndjsonPath, startedAt, endedAt); err != nil {
		c.log.Warn("failed to remove motion events from ndjson", "path", ndjsonPath, "err", err)
	}

	if c.db == nil {
		return
	}
	rel, err := filepath.Rel(c.storagePath, path)
	if err != nil {
		return
	}
	parts := strings.SplitN(filepath.ToSlash(rel), "/", 2)
	if len(parts) < 1 {
		return
	}
	cameraID := parts[0]

	// Fetch frame paths before deleting rows so we can remove the files.
	events, err := db.ListMotionEvents(c.db, cameraID, startedAt, endedAt)
	if err != nil {
		c.log.Warn("failed to list motion events for purge", "camera_id", cameraID, "err", err)
	}
	for _, ev := range events {
		if ev.FramePath == "" {
			continue
		}
		dayDir := ev.OccurredAt.UTC().Format("2006/01/02")
		jpegPath := filepath.Join(c.storagePath, cameraID, filepath.FromSlash(dayDir), ev.FramePath)
		if err := os.Remove(jpegPath); err != nil && !os.IsNotExist(err) {
			c.log.Warn("failed to delete motion jpeg", "path", jpegPath, "err", err)
		}
	}

	if err := db.DeleteMotionEventsInRange(c.db, cameraID, startedAt, endedAt); err != nil {
		c.log.Warn("failed to delete motion events from db", "camera_id", cameraID, "err", err)
	}
}

// uploadAndPurge uploads the MP4 file to the given drive, then removes the
// local file and all DB references. On upload failure nothing is deleted.
func (c *Cleaner) uploadAndPurge(drive Drive, path string, startedAt, endedAt time.Time) error {
	key := filepath.Base(filepath.Dir(path)) + "/" + filepath.Base(path)
	// Build key using camera name as first segment: "camera-name/YYYY/MM/DD/file.mp4"
	rel, err := filepath.Rel(c.storagePath, path)
	if err == nil {
		parts := strings.SplitN(filepath.ToSlash(rel), "/", 2)
		if len(parts) == 2 {
			key = c.cameraSlug(parts[0]) + "/" + parts[1]
		} else {
			key = filepath.ToSlash(rel)
		}
	}

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			// File already gone — clean up DB entry.
			_ = db.DeleteRecording(c.db, path)
			return nil
		}
		return fmt.Errorf("open %s: %w", path, err)
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return fmt.Errorf("stat %s: %w", path, err)
	}

	c.log.Info("uploading recording to drive", "path", path, "key", key)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	if err := drive.Upload(ctx, key, f, info.Size()); err != nil {
		f.Close()
		return err
	}
	f.Close()

	c.log.Debug("purging local recording after upload", "path", path)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		c.log.Warn("failed to delete recording after upload", "path", path, "err", err)
	}
	c.purgeMotionAssets(path, startedAt, endedAt)
	if err := db.DeleteRecording(c.db, path); err != nil {
		c.log.Warn("failed to delete recording from db after upload", "path", path, "err", err)
	}
	return nil
}

// cameraSlug returns the camera name slugified (lowercase, non-alphanumeric → "-").
// Falls back to the raw id if the camera is not found in the DB.
func (c *Cleaner) cameraSlug(id string) string {
	if c.db != nil {
		if cam, err := db.GetCamera(c.db, id); err == nil && cam.Name != "" {
			return slugify(cam.Name)
		}
	}
	return id
}

var deaccent = map[rune]rune{
	'à': 'a', 'á': 'a', 'â': 'a', 'ã': 'a', 'ä': 'a', 'å': 'a',
	'è': 'e', 'é': 'e', 'ê': 'e', 'ë': 'e',
	'ì': 'i', 'í': 'i', 'î': 'i', 'ï': 'i',
	'ò': 'o', 'ó': 'o', 'ô': 'o', 'õ': 'o', 'ö': 'o',
	'ù': 'u', 'ú': 'u', 'û': 'u', 'ü': 'u',
	'ç': 'c', 'ñ': 'n', 'ý': 'y',
}

func slugify(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	prev := false
	for _, r := range s {
		if mapped, ok := deaccent[r]; ok {
			r = mapped
		}
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prev = false
		} else if !prev {
			b.WriteByte('-')
			prev = true
		}
	}
	return strings.Trim(b.String(), "-")
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
		c.analyzeNewRecordings()
		c.cleanFromDB()
	} else {
		c.cleanFromFS()
	}
}

func (c *Cleaner) analyzeNewRecordings() {
	if c.db == nil {
		return
	}
	cfg, err := db.GetVideoAnalysisConfig(c.db)
	if err != nil || !cfg.Enabled || cfg.ServiceURL == "" {
		return
	}
	analyzer := c.analyzer
	if analyzer == nil {
		analyzer = analysis.NewClient(cfg.ServiceURL)
	}

	rows, err := c.db.Query(`
		SELECT r.id, r.camera_id, r.path
		FROM recordings r
		WHERE r.ended_at IS NOT NULL
		AND NOT EXISTS (SELECT 1 FROM detections d WHERE d.recording_id = r.id)`)
	if err != nil {
		c.log.Warn("analyzeNewRecordings: query failed", "err", err)
		return
	}
	type pending struct {
		id       int64
		cameraID string
		path     string
	}
	var candidates []pending
	for rows.Next() {
		var p pending
		if err := rows.Scan(&p.id, &p.cameraID, &p.path); err != nil {
			continue
		}
		candidates = append(candidates, p)
	}
	rows.Close()

	for _, p := range candidates {
		enabled, err := db.GetCameraAnalysisEnabled(c.db, p.cameraID)
		if err != nil || !enabled {
			continue
		}
		dets, err := analyzer.Analyze(context.Background(), analysis.AnalyzeRequest{
			Path:                p.path,
			Model:               cfg.Model,
			ConfidenceThreshold: cfg.ConfidenceThreshold,
		})
		if err != nil {
			c.log.Warn("analyzeNewRecordings: analyze failed", "path", p.path, "err", err)
			continue
		}
		if len(dets) == 0 {
			// Insert a sentinel so we don't retry empty results
			continue
		}
		dbDets := make([]db.Detection, len(dets))
		for i, d := range dets {
			dbDets[i] = db.Detection{Label: d.Label, Confidence: d.Confidence, FrameCount: d.FrameCount}
		}
		if err := db.InsertDetections(c.db, p.path, dbDets); err != nil {
			c.log.Warn("analyzeNewRecordings: insert detections failed", "path", p.path, "err", err)
		}
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

// effectiveInterval returns the clean interval from the DB if set, falling back
// to defaultInterval. This lets the user change the interval at runtime.
func (c *Cleaner) effectiveInterval(defaultInterval time.Duration) time.Duration {
	if c.db == nil {
		return defaultInterval
	}
	all, err := db.GetAllConfig(c.db)
	if err != nil {
		return defaultInterval
	}
	if v, ok := all["storage.interval_minutes"]; ok {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return time.Duration(n) * time.Minute
		}
	}
	return defaultInterval
}

func (c *Cleaner) Run(ctx context.Context, defaultInterval time.Duration) {
	c.Clean()
	c.CheckSize()

	// A single ticker fires every minute. syncRecordings runs on every tick;
	// the full clean runs whenever the effective interval (read from DB each tick)
	// has elapsed — so interval changes take effect within one minute.
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	lastClean := time.Now()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if c.db != nil {
				c.syncRecordings()
			}
			if time.Since(lastClean) >= c.effectiveInterval(defaultInterval) {
				c.Clean()
				c.CheckSize()
				lastClean = time.Now()
			}
		case <-c.forceCh:
			c.Clean()
			c.CheckSize()
			lastClean = time.Now()
		}
	}
}
