package database

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"
)

func setupMigrationDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}

	db.SetMaxOpenConns(1)

	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close database: %v", err)
		}
	})

	return db
}

func TestMigrate(t *testing.T) {
	db := setupMigrationDB(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("migrate database: %v", err)
	}

	// Verify the migration tracking table exists.
	var tableName string
	err := db.QueryRowContext(ctx, `
		SELECT name
		FROM sqlite_master
		WHERE type = 'table'
		AND name = 'schema_migrations'
	`).Scan(&tableName)

	if err != nil {
		t.Fatalf("schema_migrations table missing: %v", err)
	}

	// Verify the links table exists.
	err = db.QueryRowContext(ctx, `
		SELECT name
		FROM sqlite_master
		WHERE type = 'table'
		AND name = 'links'
	`).Scan(&tableName)

	if err != nil {
		t.Fatalf("links table missing: %v", err)
	}

	// Verify the expected indexes exist.
	var indexCount int
	err = db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM sqlite_master
		WHERE type = 'index'
		AND name = 'idx_links_created_at'
	`).Scan(&indexCount)

	if err != nil {
		t.Fatalf("check indexes: %v", err)
	}

	if indexCount != 1 {
		t.Errorf("expected 1 created_at index, got %d", indexCount)
	}
}

func TestMigrateRecordsVersions(t *testing.T) {
	db := setupMigrationDB(t)

	ctx := context.Background()

	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("migrate database: %v", err)
	}

	var migrationCount int
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM schema_migrations
	`).Scan(&migrationCount)

	if err != nil {
		t.Fatalf("count migrations: %v", err)
	}

	if migrationCount != 2 {
		t.Errorf("expected 2 recorded migrations, got %d", migrationCount)
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	db := setupMigrationDB(t)

	ctx := context.Background()

	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("first migration: %v", err)
	}

	var initialCount int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM schema_migrations`,
	).Scan(&initialCount); err != nil {
		t.Fatalf("count initial migrations: %v", err)
	}

	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("second migration: %v", err)
	}

	var finalCount int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM schema_migrations`,
	).Scan(&finalCount); err != nil {
		t.Fatalf("count final migrations: %v", err)
	}

	if finalCount != initialCount {
		t.Errorf("migration count changed: before %d, after %d",
			initialCount, finalCount)
	}
}

func TestMigrateWithCancelledContext(t *testing.T) {
	db := setupMigrationDB(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := Migrate(ctx, db)
	if err == nil {
		t.Fatal("expected migration to fail with cancelled context")
	}
}

func TestMigrateFailsWhenDatabaseIsClosed(t *testing.T) {
	db := setupMigrationDB(t)

	if err := db.Close(); err != nil {
		t.Fatalf("close database: %v", err)
	}

	err := Migrate(context.Background(), db)
	if err == nil {
		t.Fatal("expected migration to fail on closed database")
	}
}

func TestMigrateDoesNotRecordFailedMigration(t *testing.T) {
	db := setupMigrationDB(t)
	ctx := context.Background()

	// Create a table that conflicts with migration 002.
	// Migration 001 will succeed, but migration 002 should fail.
	_, err := db.ExecContext(ctx, `
		CREATE TABLE links (
			id INTEGER PRIMARY KEY,
			created_at DATETIME
		)
	`)
	if err != nil {
		t.Fatalf("create conflicting table: %v", err)
	}

	// Migration 001 sees the existing table and succeeds.
	// Migration 002 should fail because the expected column is missing.
	err = Migrate(ctx, db)
	if err == nil {
		t.Fatal("expected migration to fail")
	}

	// Migration 001 is recorded because it succeeded.
	var firstCount int
	err = db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM schema_migrations
		WHERE version = '001_create_links'
	`).Scan(&firstCount)

	if err != nil {
		t.Fatalf("check first migration: %v", err)
	}

	if firstCount != 1 {
		t.Errorf("expected first migration to be recorded, got %d", firstCount)
	}

	// Migration 002 must not be recorded because it failed.
	var secondCount int
	err = db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM schema_migrations
		WHERE version = '002_add_link_indexes'
	`).Scan(&secondCount)

	if err != nil {
		t.Fatalf("check second migration: %v", err)
	}

	if secondCount != 0 {
		t.Errorf("expected failed migration not to be recorded, got %d", secondCount)
	}
}

func TestMigrateErrorContainsMigrationVersion(t *testing.T) {
	db := setupMigrationDB(t)
	ctx := context.Background()

	// Migration 001 will skip table creation.
	// Migration 002 will fail because code does not exist.
	_, err := db.ExecContext(ctx, `
		CREATE TABLE links (
			id INTEGER PRIMARY KEY,
			created_at DATETIME
		)
	`)
	if err != nil {
		t.Fatalf("create conflicting table: %v", err)
	}

	err = Migrate(ctx, db)
	if err == nil {
		t.Fatal("expected migration error")
	}

	if !strings.Contains(err.Error(), "002_add_link_indexes") {
		t.Errorf("expected error to contain migration version, got %v", err)
	}
}

func TestMigrateErrorWrapping(t *testing.T) {
	db := setupMigrationDB(t)

	if err := db.Close(); err != nil {
		t.Fatalf("close database: %v", err)
	}

	err := Migrate(context.Background(), db)
	if err == nil {
		t.Fatal("expected error")
	}
}
