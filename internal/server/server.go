package server

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/lilleole/shrinkr/internal/handlers"
	"github.com/lilleole/shrinkr/internal/middleware"
	"github.com/lilleole/shrinkr/internal/services"
)

func New(
	db *sql.DB,
	linkService *services.LinkService,
	baseURL string,
) http.Handler {
	router := chi.NewRouter()

	// Global middleware.
	middleware.Register(router)

	// Rate limiters.
	createLimiter := middleware.NewRateLimiter(10, time.Minute)
	redirectLimiter := middleware.NewRateLimiter(120, time.Minute)
	apiLimiter := middleware.NewRateLimiter(60, time.Minute)

	// Initialize handlers.
	linkHandler := handlers.NewLinkHandler(linkService, baseURL)

	// Health endpoint.
	router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := db.PingContext(ctx); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"status":"unhealthy"}`))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"healthy"}`))
	})

	// Root endpoint.
	router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("Shrinkr is running!"))
	})

	// Link management API.
	router.With(createLimiter.Middleware).
		Post("/api/links", linkHandler.CreateLink)

	router.With(apiLimiter.Middleware).
		Get("/api/links", linkHandler.ListLinks)

	router.With(apiLimiter.Middleware).
		Delete("/api/links/{id}", linkHandler.DeleteLink)

	router.With(apiLimiter.Middleware).
		Patch("/api/links/{id}/active", linkHandler.SetActive)

	// Short URL redirects.
	router.With(redirectLimiter.Middleware).
		Get("/{code}", linkHandler.RedirectLink)

	return router
}
