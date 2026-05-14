package db

import (
	"fmt"
	"strconv"

	"camera/internal/config"
)

// SeedFromYAML reads the YAML at yamlPath, populates the database with all
// cameras, system_config entries and an initial admin user, then renames the
// YAML file to <yamlPath>.migrated so the seed is never applied again.
func SeedFromYAML(database *DB, yamlPath string) error {
	cfg, err := config.Load(yamlPath)
	if err != nil {
		return fmt.Errorf("seed: load yaml %q: %w", yamlPath, err)
	}

	// cameras
	for _, cam := range cfg.Cameras {
		var motion *config.MotionConfig
		if cam.Motion != nil {
			motion = cam.Motion
		}
		if err := CreateCamera(database, cam, motion); err != nil {
			return fmt.Errorf("seed: insert camera %q: %w", cam.ID, err)
		}
	}

	// system_config
	pairs := configPairs(cfg)
	for k, v := range pairs {
		if err := SetConfig(database, k, v); err != nil {
			return fmt.Errorf("seed: set config %q: %w", k, err)
		}
	}

	// admin user
	username := cfg.Server.Username
	if username == "" {
		username = "admin"
	}
	password := cfg.Server.Password
	if password == "" {
		password = "changeme"
	}
	if _, err := CreateUser(database, username, password, "admin"); err != nil {
		return fmt.Errorf("seed: create admin user: %w", err)
	}

	return nil
}

// configPairs converts the relevant Config fields to system_config key-value pairs.
func configPairs(cfg config.Config) map[string]string {
	withMotion, withoutMotion := cfg.Storage.EffectiveRetention()

	p := map[string]string{
		"debug":    strconv.FormatBool(cfg.Debug),
		"timezone": cfg.Timezone,

		"log.output": cfg.Log.Output,
		"log.path":   cfg.Log.Path,

		"server.port":             strconv.Itoa(cfg.Server.Port),
		"server.segments_path":    cfg.Server.SegmentsPath,
		"server.hls_dvr_seconds":  strconv.Itoa(cfg.Server.HLSDVRSeconds),
		"server.jwt_secret":       cfg.Server.JWTSecret,

		"storage.path":                     cfg.Storage.Path,
		"storage.with_motion_minutes":      strconv.Itoa(withMotion),
		"storage.without_motion_minutes":   strconv.Itoa(withoutMotion),
		"storage.interval_minutes":         strconv.Itoa(cfg.Storage.IntervalMinutes),
		"storage.max_size_gb":              strconv.FormatFloat(cfg.Storage.MaxSizeGB, 'f', -1, 64),
		"storage.warn_percent":             strconv.FormatFloat(cfg.Storage.WarnPercent, 'f', -1, 64),

		"motion.enabled":          strconv.FormatBool(cfg.Motion.Enabled),
		"motion.threshold":        strconv.FormatFloat(cfg.Motion.Threshold, 'f', -1, 64),
		"motion.fps":              strconv.Itoa(cfg.Motion.FPS),
		"motion.cooldown_seconds": strconv.Itoa(cfg.Motion.CooldownSeconds),
	}

	return p
}
