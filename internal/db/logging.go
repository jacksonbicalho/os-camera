package db

import (
	"database/sql"
	"log/slog"
	"strings"
	"time"
)

const slowQueryThreshold = 100 * time.Millisecond
const maxSQLLen = 120

// WithLogger attaches a logger to the DB. Returns the receiver for chaining.
func (d *DB) WithLogger(log *slog.Logger) *DB {
	d.log = log
	return d
}

// Exec shadows sql.DB.Exec to add query logging.
func (d *DB) Exec(query string, args ...any) (sql.Result, error) {
	start := time.Now()
	result, err := d.DB.Exec(query, args...)
	d.logQuery(query, time.Since(start), err)
	return result, err
}

// Query shadows sql.DB.Query to add query logging.
func (d *DB) Query(query string, args ...any) (*sql.Rows, error) {
	start := time.Now()
	rows, err := d.DB.Query(query, args...)
	d.logQuery(query, time.Since(start), err)
	return rows, err
}

// QueryRow shadows sql.DB.QueryRow to add query logging.
// Scan errors are not visible here — only timing and SQL are logged.
func (d *DB) QueryRow(query string, args ...any) *sql.Row {
	start := time.Now()
	row := d.DB.QueryRow(query, args...)
	d.logQuery(query, time.Since(start), nil)
	return row
}

func (d *DB) logQuery(query string, dur time.Duration, err error) {
	if d.log == nil {
		return
	}

	sql := truncate(strings.Join(strings.Fields(query), " "), maxSQLLen)
	durStr := dur.Round(time.Microsecond).String()

	switch {
	case err != nil:
		d.log.Error("db query", "sql", sql, "duration", durStr, "err", err)
	case dur >= slowQueryThreshold:
		d.log.Warn("db query slow", "sql", sql, "duration", durStr)
	}
	// Fast successful queries are not logged — even in debug mode they are too
	// frequent (every poll cycle, every motion event) to be useful.
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
