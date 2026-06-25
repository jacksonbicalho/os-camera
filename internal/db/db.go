package db

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// DB wraps *sql.DB and exposes whether it was freshly created.
type DB struct {
	*sql.DB
	IsNew bool
	log   *slog.Logger
}

// Open opens (or creates) the SQLite database at path, applies pending
// migrations and returns the DB handle. IsNew is true when the file did
// not exist before this call.
func Open(path string) (*DB, error) {
	_, statErr := os.Stat(path)
	isNew := os.IsNotExist(statErr)

	sqlDB, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %q: %w", path, err)
	}

	// SQLite performs best with a single writer connection.
	sqlDB.SetMaxOpenConns(1)

	if _, err := sqlDB.Exec(`PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON;`); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("pragma setup: %w", err)
	}

	if err := applyMigrations(sqlDB); err != nil {
		sqlDB.Close()
		return nil, err
	}

	return &DB{DB: sqlDB, IsNew: isNew}, nil
}

// HasPendingMigrations abre o banco em path e reporta se há alguma migration
// ainda não registrada em schema_migrations — sem aplicá-las. Útil para decidir
// um backup antes do Open (que aplica as pendentes). Em um banco inexistente,
// todas as migrations contam como pendentes.
func HasPendingMigrations(path string) (bool, error) {
	// Banco inexistente = todas as migrations pendentes. Retorna sem abrir/criar
	// o arquivo — criá-lo aqui faria o Open posterior achar que o banco não é
	// novo e pular o seed do admin.
	if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
		return true, nil
	}

	d, err := sql.Open("sqlite", path)
	if err != nil {
		return false, fmt.Errorf("open sqlite %q: %w", path, err)
	}
	defer d.Close()

	if _, err := d.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY)`); err != nil {
		return false, fmt.Errorf("create schema_migrations: %w", err)
	}

	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return false, fmt.Errorf("read migrations dir: %w", err)
	}

	var applied int
	if err := d.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&applied); err != nil {
		return false, fmt.Errorf("count applied migrations: %w", err)
	}
	return applied < len(entries), nil
}

func applyMigrations(db *sql.DB) error {
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY
		)
	`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for i, entry := range entries {
		version := i + 1

		var applied int
		err := db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version=?", version).Scan(&applied)
		if err != nil {
			return fmt.Errorf("check migration %d: %w", version, err)
		}
		if applied > 0 {
			continue
		}

		data, err := migrationsFS.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}

		statements := splitSQL(string(data))
		for _, stmt := range statements {
			if _, err := db.Exec(stmt); err != nil {
				return fmt.Errorf("execute migration %s: %w", entry.Name(), err)
			}
		}

		if _, err := db.Exec("INSERT INTO schema_migrations(version) VALUES(?)", version); err != nil {
			return fmt.Errorf("record migration %d: %w", version, err)
		}
	}

	return nil
}

// splitSQL splits a SQL script into individual statements, ignoring empty ones.
func splitSQL(script string) []string {
	parts := strings.Split(script, ";")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			result = append(result, s)
		}
	}
	return result
}
