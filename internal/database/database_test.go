package database

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestOpenCreatesDatabaseAndDirectory(t *testing.T) {
	databasePath := filepath.Join(
		t.TempDir(),
		"nested",
		"directory",
		"shrinkr.db",
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	db, err := Open(ctx, databasePath)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	if _, err := os.Stat(databasePath); err != nil {
		t.Fatalf("expected database file to exist: %v", err)
	}

	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping database: %v", err)
	}
}

func TestOpenRunsMigrations(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "shrinkr.db")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	db, err := Open(ctx, databasePath)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	var tableName string

	err = db.QueryRowContext(
		ctx,
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'links'`,
	).Scan(&tableName)

	if err != nil {
		t.Fatalf("expected links table to exist: %v", err)
	}

	if tableName != "links" {
		t.Errorf("expected table name 'links', got %q", tableName)
	}
}

func TestOpenCanReopenExistingDatabase(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "shrinkr.db")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	db, err := Open(ctx, databasePath)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}

	if err := db.Close(); err != nil {
		t.Fatalf("close database: %v", err)
	}

	db, err = Open(ctx, databasePath)
	if err != nil {
		t.Fatalf("second open: %v", err)
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping reopened database: %v", err)
	}
}

func TestOpenFailsWhenDatabasePathIsDirectory(t *testing.T) {
	databasePath := t.TempDir()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	db, err := Open(ctx, databasePath)
	if err == nil {
		if db != nil {
			db.Close()
		}
		t.Fatal("expected error when database path is a directory")
	}
}

func TestOpenFailsWithCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	databasePath := filepath.Join(t.TempDir(), "shrinkr.db")

	_, err := Open(ctx, databasePath)
	if err == nil {
		t.Fatal("expected error with cancelled context")
	}
}
