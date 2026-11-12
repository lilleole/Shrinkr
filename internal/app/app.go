package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/lilleole/shrinkr/internal/config"
	"github.com/lilleole/shrinkr/internal/database"
	"github.com/lilleole/shrinkr/internal/handlers"
	"github.com/lilleole/shrinkr/internal/middleware"
	"github.com/lilleole/shrinkr/internal/services"
)

func Run(ctx context.Context) error {
	cfg := config.Load()

	// Open database and automatically apply migrations.
	db, err := database.Open(ctx, cfg.DatabasePath)
	if err != nil {
		return err
	}
	defer db.Close()

	log.Println("Database connected and migrations applied")

	// Initialize application dependencies.
	linkRepository := database.NewLinkRepository(db)
	linkService := services.NewLinkService(
		linkRepository,
		cfg.DefaultExpirationDays,
	)

	// Configure HTTP router.
	router := chi.NewRouter()
	middleware.Register(router)

	// Health endpoint.
	router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		requestCtx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := db.PingContext(requestCtx); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"status": "unhealthy",
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status": "healthy",
		})
	})

	// Register the dashboard, management API and redirect routes.
	handlers.RegisterRoutes(router, linkService, cfg.BaseURL)

	// Configure HTTP server.
	server := &http.Server{
		Addr:              cfg.Address,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	serverErrors := make(chan error, 1)

	go func() {
		log.Printf("Shrinkr listening on %s", cfg.Address)
		serverErrors <- server.ListenAndServe()
	}()

	// Wait for shutdown signal or server failure.
	select {
	case err := <-serverErrors:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("HTTP server: %w", err)
		}
		return nil

	case <-ctx.Done():
		log.Println("Shutting down Shrinkr...")

		shutdownCtx, cancel := context.WithTimeout(
			context.Background(),
			10*time.Second,
		)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			_ = server.Close()
			return fmt.Errorf("HTTP server shutdown: %w", err)
		}

		return nil
	}
}

// Keep database/sql imported in this package's public startup signature context.
var _ *sql.DB
