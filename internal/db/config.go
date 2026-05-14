package db

import (
	"database/sql"
	"errors"
	"fmt"
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
