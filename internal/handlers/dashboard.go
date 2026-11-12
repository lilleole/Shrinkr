package handlers

import (
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/lilleole/shrinkr/internal/models"
	"github.com/lilleole/shrinkr/internal/services"
	"github.com/lilleole/shrinkr/web"
)

type DashboardHandler struct {
	service   *services.LinkService
	baseURL   string
	templates *template.Template
}

type dashboardData struct {
	BaseURL     string
	Links       []models.Link
	TotalLinks  int
	ActiveLinks int
	TotalClicks int64
}

type messageData struct {
	Text  string
	Error bool
}

func NewDashboardHandler(service *services.LinkService, baseURL string) *DashboardHandler {
	templates := template.Must(
		template.New("dashboard.html").ParseFS(web.Files, "templates/dashboard.html"),
	)

	return &DashboardHandler{
		service:   service,
		baseURL:   strings.TrimRight(baseURL, "/"),
		templates: templates,
	}
}

func (h *DashboardHandler) Index(w http.ResponseWriter, r *http.Request) {
	data, err := h.loadDashboardData(r)
	if err != nil {
		http.Error(w, "Failed to load dashboard", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.templates.ExecuteTemplate(w, "dashboard", data); err != nil {
		http.Error(w, "Failed to render dashboard", http.StatusInternalServerError)
	}
}

func (h *DashboardHandler) List(w http.ResponseWriter, r *http.Request) {
	data, err := h.loadDashboardData(r)
	if err != nil {
		http.Error(w, "Failed to load links", http.StatusInternalServerError)
		return
	}

	h.renderLinks(w, data)
}

func (h *DashboardHandler) Create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.renderMessage(w, "Invalid form submission.", true)
		return
	}

	targetURL := strings.TrimSpace(r.FormValue("target_url"))
	if targetURL == "" {
		h.renderMessage(w, "Enter a destination URL.", true)
		return
	}

	var expiresAt *time.Time
	if raw := strings.TrimSpace(r.FormValue("expires_at")); raw != "" {
		parsed, err := time.ParseInLocation("2006-01-02T15:04", raw, time.Local)
		if err != nil {
			h.renderMessage(w, "The expiration date is invalid.", true)
			return
		}
		expiresAt = &parsed
	}

	if _, err := h.service.CreateWithExpiration(r.Context(), targetURL, expiresAt); err != nil {
		h.renderMessage(w, "Could not create the link. Check that the URL starts with http:// or https:// and that the expiration is in the future.", true)
		return
	}

	w.Header().Set("HX-Trigger", "linksChanged")
	h.renderMessage(w, "Short link created successfully.", false)
}

func (h *DashboardHandler) SetActive(w http.ResponseWriter, r *http.Request) {
	id, err := dashboardLinkID(r)
	if err != nil {
		http.Error(w, "Invalid link ID", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form submission", http.StatusBadRequest)
		return
	}

	active := r.FormValue("active") == "true"
	if err := h.service.SetActive(r.Context(), id, active); err != nil {
		http.Error(w, "Could not update link", http.StatusInternalServerError)
		return
	}

	h.List(w, r)
}

func (h *DashboardHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := dashboardLinkID(r)
	if err != nil {
		http.Error(w, "Invalid link ID", http.StatusBadRequest)
		return
	}

	if err := h.service.Delete(r.Context(), id); err != nil {
		http.Error(w, "Could not delete link", http.StatusInternalServerError)
		return
	}

	h.List(w, r)
}

func dashboardLinkID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, strconv.ErrSyntax
	}
	return id, nil
}

func (h *DashboardHandler) loadDashboardData(r *http.Request) (dashboardData, error) {
	links, err := h.service.List(r.Context())
	if err != nil {
		return dashboardData{}, err
	}
	if links == nil {
		links = []models.Link{}
	}

	data := dashboardData{
		BaseURL:    h.baseURL,
		Links:      links,
		TotalLinks: len(links),
	}
	for _, link := range links {
		data.TotalClicks += link.Clicks
		if link.Active && (link.ExpiresAt == nil || link.ExpiresAt.After(time.Now())) {
			data.ActiveLinks++
		}
	}
	return data, nil
}

func (h *DashboardHandler) renderLinks(w http.ResponseWriter, data dashboardData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.templates.ExecuteTemplate(w, "links", data); err != nil {
		http.Error(w, "Failed to render links", http.StatusInternalServerError)
	}
}

func (h *DashboardHandler) renderMessage(w http.ResponseWriter, message string, isError bool) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.templates.ExecuteTemplate(w, "form-message", messageData{
		Text:  message,
		Error: isError,
	}); err != nil {
		http.Error(w, "Failed to render response", http.StatusInternalServerError)
	}
}
