package db

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
)

// GetConfig returns the value for the given key from system_config.
// Returns an error if the key does not exist.
func GetConfig(db *DB, key string) (string, error) {
	var val string
	err := db.QueryRow(`SELECT value FROM system_config WHERE key=?`, key).Scan(&val)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("config key %q not found", key)
		}
		return "", fmt.Errorf("get config %q: %w", key, err)
	}
	return val, nil
}

// SetConfig inserts or replaces the value for the given key.
func SetConfig(db *DB, key, value string) error {
	_, err := db.Exec(
		`INSERT INTO system_config(key, value) VALUES(?,?)
		 ON CONFLICT(key) DO UPDATE SET value=excluded.value`,
		key, value,
	)
	if err != nil {
		return fmt.Errorf("set config %q: %w", key, err)
	}
	return nil
}

// GetAllConfig returns all key-value pairs from system_config.
func GetAllConfig(db *DB) (map[string]string, error) {
	rows, err := db.Query(`SELECT key, value FROM system_config ORDER BY key`)
	if err != nil {
		return nil, fmt.Errorf("get all config: %w", err)
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, fmt.Errorf("scan config row: %w", err)
		}
		result[k] = v
	}
	return result, rows.Err()
}

// ResolvedStorageSettings holds the effective storage settings read from the database.
type ResolvedStorageSettings struct {
	WithMotionMinutes    int
	WithoutMotionMinutes int
	IntervalMinutes      int
	MaxSizeGB            float64
	WarnPercent          float64
}

// DefaultStorageSettings are the hardcoded defaults used when a key is absent from the database.
var DefaultStorageSettings = ResolvedStorageSettings{
	WithMotionMinutes:    10080, // 7 days
	WithoutMotionMinutes: 1440,  // 1 day
	IntervalMinutes:      60,
	MaxSizeGB:            0,  // unlimited
	WarnPercent:          70,
}

// StorageSettingsFromDB reads storage settings from the database, falling back to
// DefaultStorageSettings for any key that is missing or unparseable.
// If database is nil, returns DefaultStorageSettings.
func StorageSettingsFromDB(database *DB) ResolvedStorageSettings {
	result := DefaultStorageSettings
	if database == nil {
		return result
	}
	all, err := GetAllConfig(database)
	if err != nil {
		return result
	}
	if v, ok := all["storage.with_motion_minutes"]; ok {
		if n, err := strconv.Atoi(v); err == nil {
			result.WithMotionMinutes = n
		}
	}
	if v, ok := all["storage.without_motion_minutes"]; ok {
		if n, err := strconv.Atoi(v); err == nil {
			result.WithoutMotionMinutes = n
		}
	}
	if v, ok := all["storage.interval_minutes"]; ok {
		if n, err := strconv.Atoi(v); err == nil {
			result.IntervalMinutes = n
		}
	}
	if v, ok := all["storage.max_size_gb"]; ok {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			result.MaxSizeGB = f
		}
	}
	if v, ok := all["storage.warn_percent"]; ok {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			result.WarnPercent = f
		}
	}
	return result
}

// EnsureStorageDefaults writes DefaultStorageSettings into the database for any key
// that does not yet exist. Safe to call on every startup — never overwrites existing values.
func EnsureStorageDefaults(database *DB) error {
	d := DefaultStorageSettings
	pairs := map[string]string{
		"storage.with_motion_minutes":    strconv.Itoa(d.WithMotionMinutes),
		"storage.without_motion_minutes": strconv.Itoa(d.WithoutMotionMinutes),
		"storage.interval_minutes":       strconv.Itoa(d.IntervalMinutes),
		"storage.max_size_gb":            strconv.FormatFloat(d.MaxSizeGB, 'f', -1, 64),
		"storage.warn_percent":           strconv.FormatFloat(d.WarnPercent, 'f', -1, 64),
	}
	for k, v := range pairs {
		if _, err := database.Exec(
			`INSERT OR IGNORE INTO system_config(key, value) VALUES(?,?)`, k, v,
		); err != nil {
			return fmt.Errorf("ensure storage default %q: %w", k, err)
		}
	}
	return nil
}
