package database

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func Migrate(db *sql.DB) error {
	_, err := db.Exec(`
			CREATE TABLE IF NOT EXISTS schema_migrations (
				version TEXT PRIMARY KEY,
				applied_at TIMESTAMP DEFAULT NOW()
			)
	`)

	if err != nil {
		return fmt.Errorf("creating migrations table: %w", err)
	}

	files, err := filepath.Glob("migrations/*.up.sql")
	if err != nil {
		return fmt.Errorf("reading migration files: %w", err)
	}

	sort.Strings(files)

	for _, file := range files {
		version := strings.TrimSuffix(filepath.Base(file), ".up.sql")

		var exists bool
		err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)", version).Scan(&exists)
		if err != nil {
			return fmt.Errorf("checking migration %s: %w", version, err)
		}

		if exists {
			continue
		}

		content, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("reading migration %s: %w", file, err)
		}

		slog.Info("executing migration", "version", version)

		_, err = db.Exec(string(content))
		if err != nil {
			return fmt.Errorf("executing migration %s: %w", version, err)
		}

		_, err = db.Exec("INSERT INTO schema_migrations (version) VALUES ($1)", version)
		if err != nil {
			return fmt.Errorf("recording migration %s: %w", version, err)
		}

		slog.Info("applied migration", "version", version)
	}

	return nil
}
