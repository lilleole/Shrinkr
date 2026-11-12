package middleware

import (
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
)

// Register configures the application's HTTP middleware.
func Register(router chi.Router) {
	router.Use(
		chiMiddleware.RequestID,
		chiMiddleware.Logger,
		chiMiddleware.Recoverer,
	)
}
