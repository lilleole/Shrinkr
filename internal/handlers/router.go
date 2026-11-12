package handlers

import (
	"github.com/go-chi/chi/v5"
	"github.com/lilleole/shrinkr/internal/services"
)

func RegisterRoutes(
	router chi.Router,
	linkService *services.LinkService,
	baseURL string,
) {
	linkHandler := NewLinkHandler(linkService, baseURL)

	// Link management API
	router.Get("/api/links", linkHandler.ListLinks)
	router.Post("/api/links", linkHandler.CreateLink)
	router.Delete("/api/links/{id}", linkHandler.DeleteLink)
	router.Patch("/api/links/{id}/active", linkHandler.SetActive)

	// Short URL redirect
	router.Get("/{code}", linkHandler.RedirectLink)
}
