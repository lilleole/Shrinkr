package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/lilleole/shrinkr/internal/database"
	"github.com/lilleole/shrinkr/internal/models"
	"github.com/lilleole/shrinkr/internal/services"
)

type LinkHandler struct {
	service *services.LinkService
	baseURL string
}

func NewLinkHandler(service *services.LinkService, baseURL string) *LinkHandler {
	return &LinkHandler{
		service: service,
		baseURL: strings.TrimRight(baseURL, "/"),
	}
}

// CreateLinkRequest represents the JSON body for creating a link.
type CreateLinkRequest struct {
	TargetURL string  `json:"target_url"`
	ExpiresAt *string `json:"expires_at,omitempty"`
}

// CreateLink creates a new shortened URL.
func (h *LinkHandler) CreateLink(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var req CreateLinkRequest

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	var expiresAt *time.Time

	if req.ExpiresAt != nil && *req.ExpiresAt != "" {
		parsed, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Invalid expiration date. Use RFC3339 format.")
			return
		}

		expiresAt = &parsed
	}

	link, err := h.service.CreateWithExpiration(
		r.Context(),
		req.TargetURL,
		expiresAt,
	)
	if err != nil {
		if errors.Is(err, services.ErrInvalidURL) {
			writeError(w, http.StatusBadRequest, "Invalid destination URL")
			return
		}

		writeError(w, http.StatusInternalServerError, "Failed to create link")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"link":      link,
		"short_url": fmt.Sprintf("%s/%s", h.baseURL, link.Code),
	})
}

// RedirectLink redirects a short URL to its destination.
func (h *LinkHandler) RedirectLink(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")

	link, err := h.service.Get(r.Context(), code)
	if err != nil {
		if errors.Is(err, database.ErrLinkNotFound) {
			http.NotFound(w, r)
			return
		}

		writeError(w, http.StatusInternalServerError, "Failed to retrieve link")
		return
	}

	if err := h.service.RecordClick(r.Context(), link.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to record click")
		return
	}

	http.Redirect(w, r, link.TargetURL, http.StatusFound)
}

// ListLinks returns all shortened links.
func (h *LinkHandler) ListLinks(w http.ResponseWriter, r *http.Request) {
	links, err := h.service.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to retrieve links")
		return
	}

	if links == nil {
		links = []models.Link{}
	}

	writeJSON(w, http.StatusOK, links)
}

// DeleteLink deletes a shortened link.
func (h *LinkHandler) DeleteLink(w http.ResponseWriter, r *http.Request) {
	id, err := parseLinkID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid link ID")
		return
	}

	err = h.service.Delete(r.Context(), id)
	if err != nil {
		if errors.Is(err, database.ErrLinkNotFound) {
			http.NotFound(w, r)
			return
		}

		writeError(w, http.StatusInternalServerError, "Failed to delete link")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// SetActiveRequest represents the request to enable or disable a link.
type SetActiveRequest struct {
	Active bool `json:"active"`
}

// SetActive enables or disables a shortened link.
func (h *LinkHandler) SetActive(w http.ResponseWriter, r *http.Request) {
	id, err := parseLinkID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid link ID")
		return
	}

	var req SetActiveRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.service.SetActive(r.Context(), id, req.Active); err != nil {
		if errors.Is(err, database.ErrLinkNotFound) {
			http.NotFound(w, r)
			return
		}

		writeError(w, http.StatusInternalServerError, "Failed to update link")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":     id,
		"active": req.Active,
	})
}

// parseLinkID extracts and validates the link ID from the URL.
func parseLinkID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("invalid link ID")
	}

	return id, nil
}

// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		// The response may already have been partially written.
		return
	}
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{
		"error": message,
	})
}
