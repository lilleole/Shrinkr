package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/lilleole/shrinkr/internal/services"
	"github.com/lilleole/shrinkr/web"
)

func RegisterRoutes(
	router chi.Router,
	linkService *services.LinkService,
	baseURL string,
) {
	dashboardHandler := NewDashboardHandler(linkService, baseURL)
	linkHandler := NewLinkHandler(linkService, baseURL)

	// Static assets are embedded in the binary, so Docker does not need a web directory.
	staticFiles := http.FileServer(http.FS(web.StaticFiles()))
	router.Handle("/static/*", http.StripPrefix("/static/", staticFiles))

	// Dashboard and HTML actions.
	router.Get("/", dashboardHandler.Index)
	router.Get("/ui/links", dashboardHandler.List)
	router.Post("/ui/links", dashboardHandler.Create)
	router.Post("/ui/links/{id}/active", dashboardHandler.SetActive)
	router.Post("/ui/links/{id}/delete", dashboardHandler.Delete)

	// JSON API.
	router.Get("/api/links", linkHandler.ListLinks)
	router.Post("/api/links", linkHandler.CreateLink)
	router.Delete("/api/links/{id}", linkHandler.DeleteLink)
	router.Patch("/api/links/{id}/active", linkHandler.SetActive)

	// Short URL redirect. Keep this last because it matches any single path segment.
	router.Get("/{code}", linkHandler.RedirectLink)
}
