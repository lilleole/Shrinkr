package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	_ "modernc.org/sqlite"

	"github.com/lilleole/shrinkr/internal/config"
	"github.com/lilleole/shrinkr/internal/database"
	"github.com/lilleole/shrinkr/internal/handlers"
	"github.com/lilleole/shrinkr/internal/services"
)

func main() {
	// Load configuration from environment variables.
	cfg := config.Load()

	// Ensure the directory for the SQLite database exists.
	if err := os.MkdirAll(filepath.Dir(cfg.DatabasePath), 0755); err != nil {
		log.Fatalf("Failed to create database directory: %v", err)
	}

	// Connect to SQLite.
	db, err := sql.Open("sqlite", cfg.DatabasePath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// SQLite works best with a single connection for this setup.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Configure SQLite.
	pragmas := []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA journal_mode = WAL",
		"PRAGMA busy_timeout = 5000",
	}

	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			log.Fatalf("Failed to configure SQLite (%s): %v", pragma, err)
		}
	}

	log.Printf("Connected to SQLite database: %s", cfg.DatabasePath)

	// Initialize application services.
	linkRepository := database.NewLinkRepository(db)
	linkService := services.NewLinkService(
		linkRepository,
		cfg.DefaultExpirationDays,
	)

	// Set up the HTTP router.
	router := chi.NewRouter()

	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)

	// Health check endpoint.
	router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := db.PingContext(ctx); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"status": "unhealthy",
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status": "healthy",
		})
	})

	// Root endpoint.
	router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("Shrinkr is running!"))
	})

	// Register API and redirect routes.
	handlers.RegisterRoutes(router, linkService, cfg.BaseURL)

	// Configure HTTP server.
	server := &http.Server{
		Addr:              cfg.Address,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Start server in a separate goroutine.
	go func() {
		log.Printf("Shrinkr listening on %s", cfg.Address)

		if err := server.ListenAndServe(); err != nil &&
			!errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	// Wait for an interrupt or termination signal.
	stop, cancel := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer cancel()

	<-stop.Done()
	log.Println("Shutting down Shrinkr...")

	// Give active requests time to finish.
	ctx, shutdownCancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer shutdownCancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Graceful shutdown failed: %v", err)

		if err := server.Close(); err != nil {
			log.Printf("Failed to close HTTP server: %v", err)
		}
	}

	log.Println("Shrinkr stopped.")
}
