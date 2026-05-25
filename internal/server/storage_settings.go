package server

import (
	"encoding/json"
	"net/http"
	"strconv"

	"camera/internal/db"
)

const (
	keyWithMotionMinutes    = "storage.with_motion_minutes"
	keyWithoutMotionMinutes = "storage.without_motion_minutes"
	keyIntervalMinutes      = "storage.interval_minutes"
	keyMaxSizeGB            = "storage.max_size_gb"
	keyWarnPercent          = "storage.warn_percent"
)

// effectiveStorageSettings returns the active storage settings, preferring
// DB overrides over the values loaded from camera.yaml at startup.
func (s *Server) effectiveStorageSettings() (withMotion, withoutMotion, interval int, maxGB, warnPct float64) {
	cfgWithMotion, cfgWithoutMotion := s.storageCfg.EffectiveRetention()
	withMotion = cfgWithMotion
	withoutMotion = cfgWithoutMotion
	interval = s.storageCfg.IntervalMinutes
	maxGB = s.storageCfg.MaxSizeGB
	warnPct = s.storageCfg.WarnPercent

	if s.db == nil {
		return
	}
	all, err := db.GetAllConfig(s.db)
	if err != nil {
		return
	}
	if v, ok := all[keyWithMotionMinutes]; ok {
		if n, err := strconv.Atoi(v); err == nil {
			withMotion = n
		}
	}
	if v, ok := all[keyWithoutMotionMinutes]; ok {
		if n, err := strconv.Atoi(v); err == nil {
			withoutMotion = n
		}
	}
	if v, ok := all[keyIntervalMinutes]; ok {
		if n, err := strconv.Atoi(v); err == nil {
			interval = n
		}
	}
	if v, ok := all[keyMaxSizeGB]; ok {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			maxGB = f
		}
	}
	if v, ok := all[keyWarnPercent]; ok {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			warnPct = f
		}
	}
	return
}

func (s *Server) handleUpdateStorageSettings(w http.ResponseWriter, r *http.Request) {
	if !s.requireDB(w) {
		return
	}
	var input struct {
		WithMotionMinutes    *int     `json:"with_motion_minutes"`
		WithoutMotionMinutes *int     `json:"without_motion_minutes"`
		IntervalMinutes      *int     `json:"interval_minutes"`
		MaxSizeGB            *float64 `json:"max_size_gb"`
		WarnPercent          *float64 `json:"warn_percent"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	set := func(key string, val any) error {
		return db.SetConfig(s.db, key, strconv.FormatFloat(toFloat(val), 'f', -1, 64))
	}
	if input.WithMotionMinutes != nil {
		if err := db.SetConfig(s.db, keyWithMotionMinutes, strconv.Itoa(*input.WithMotionMinutes)); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	}
	if input.WithoutMotionMinutes != nil {
		if err := db.SetConfig(s.db, keyWithoutMotionMinutes, strconv.Itoa(*input.WithoutMotionMinutes)); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	}
	if input.IntervalMinutes != nil {
		if err := db.SetConfig(s.db, keyIntervalMinutes, strconv.Itoa(*input.IntervalMinutes)); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	}
	if input.MaxSizeGB != nil {
		if err := set(keyMaxSizeGB, *input.MaxSizeGB); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	}
	if input.WarnPercent != nil {
		if err := set(keyWarnPercent, *input.WarnPercent); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	}
	// Trigger an immediate clean so new retention settings take effect right away.
	if s.cleaner != nil {
		s.cleaner.ForceClean()
	}
	wm, wom, interval, maxGB, warnPct := s.effectiveStorageSettings()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"with_motion_minutes":    wm,
		"without_motion_minutes": wom,
		"interval_minutes":       interval,
		"max_size_gb":            maxGB,
		"warn_percent":           warnPct,
	})
}

func toFloat(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case *float64:
		if x != nil {
			return *x
		}
	}
	return 0
}
