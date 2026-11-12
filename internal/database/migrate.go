package database

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

func Migrate(ctx context.Context, db *sql.DB) error {
	// Create migration tracking table.
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("create migration table: %w", err)
	}

	// Read embedded migration files.
	files, err := fs.Glob(migrationFiles, "migrations/*.sql")
	if err != nil {
		return fmt.Errorf("find migrations: %w", err)
	}

	// Ensure migrations are executed in order.
	sort.Strings(files)

	for _, filename := range files {
		version := strings.TrimSuffix(path.Base(filename), ".sql")

		// Check whether this migration has already been applied.
		var exists bool
		err := db.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT 1
				FROM schema_migrations
				WHERE version = ?
			)
		`, version).Scan(&exists)

		if err != nil {
			return fmt.Errorf("check migration %s: %w", version, err)
		}

		if exists {
			continue
		}

		// Read migration SQL.
		query, err := migrationFiles.ReadFile(filename)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", version, err)
		}

		// Run migration and record it atomically.
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin migration %s: %w", version, err)
		}

		if _, err := tx.ExecContext(ctx, string(query)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("execute migration %s: %w", version, err)
		}

		_, err = tx.ExecContext(ctx, `
			INSERT INTO schema_migrations (version)
			VALUES (?)
		`, version)

		if err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %s: %w", version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s: %w", version, err)
		}

		fmt.Printf("Applied migration: %s\n", version)
	}

	return nil
}
