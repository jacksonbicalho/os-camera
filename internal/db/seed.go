package db

import (
	"fmt"
	"strconv"

	"camera/internal/config"
)

// SeedFromBootstrap creates the initial admin user from the bootstrap config.
// It is called once when the database is new. The admin is created with
// must_change_password=true so the user is forced to change the password on
// first login.
func SeedFromBootstrap(database *DB, cfg config.Config) error {
	username := cfg.Admin.Username
	if username == "" {
		username = "admin"
	}
	password := cfg.Admin.Password
	if password == "" {
		password = "changeme"
	}
	if _, err := CreateUser(database, username, password, "admin", true); err != nil {
		return fmt.Errorf("seed: create admin user: %w", err)
	}

	pairs := bootstrapConfigPairs(cfg)
	for k, v := range pairs {
		if err := SetConfig(database, k, v); err != nil {
			return fmt.Errorf("seed: set config %q: %w", k, err)
		}
	}

	return nil
}

func bootstrapConfigPairs(cfg config.Config) map[string]string {
	return map[string]string{
		"debug":    strconv.FormatBool(cfg.Debug),
		"timezone": cfg.Timezone,

		"log.output": cfg.Log.Output,
		"log.path":   cfg.Log.Path,

		"server.port":            strconv.Itoa(cfg.Server.Port),
		"server.segments_path":   cfg.Server.SegmentsPath,
		"server.hls_dvr_seconds": strconv.Itoa(cfg.Server.HLSDVRSeconds),
		"server.jwt_secret":      cfg.Server.JWTSecret,

		"storage.path": cfg.Storage.Path,
	}
}
