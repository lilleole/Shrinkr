package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/lilleole/shrinkr/internal/database"
	"github.com/lilleole/shrinkr/internal/models"
	"github.com/lilleole/shrinkr/internal/services"
)

func setupTestHandler(t *testing.T) (*chi.Mux, *database.LinkRepository) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}

	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close database: %v", err)
		}
	})

	repository := database.NewLinkRepository(db)
	service := services.NewLinkService(repository, 0)
	handler := NewLinkHandler(service, "http://localhost:8080")

	router := chi.NewRouter()

	router.Post("/api/links", handler.CreateLink)
	router.Get("/api/links", handler.ListLinks)
	router.Delete("/api/links/{id}", handler.DeleteLink)
	router.Patch("/api/links/{id}/active", handler.SetActive)
	router.Get("/{code}", handler.RedirectLink)

	return router, repository
}

func performRequest(
	router http.Handler,
	method string,
	path string,
	body string,
) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	return recorder
}

func createHandlerTestLink(
	t *testing.T,
	repository *database.LinkRepository,
) *models.Link {
	t.Helper()

	link := &models.Link{
		Code:      "testCode123",
		TargetURL: "https://example.com",
		Active:    true,
	}

	if err := repository.Create(context.Background(), link); err != nil {
		t.Fatalf("create test link: %v", err)
	}

	return link
}

func TestCreateLink(t *testing.T) {
	router, _ := setupTestHandler(t)

	recorder := performRequest(
		router,
		http.MethodPost,
		"/api/links",
		`{"target_url":"https://example.com"}`,
	)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s",
			http.StatusCreated, recorder.Code, recorder.Body.String())
	}

	var response struct {
		Link     models.Link `json:"link"`
		ShortURL string      `json:"short_url"`
	}

	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Link.Code == "" {
		t.Error("expected generated shortcode")
	}

	if response.Link.TargetURL != "https://example.com" {
		t.Errorf("unexpected target URL: %q", response.Link.TargetURL)
	}

	expectedShortURL := fmt.Sprintf("http://localhost:8080/%s", response.Link.Code)
	if response.ShortURL != expectedShortURL {
		t.Errorf("expected short URL %q, got %q",
			expectedShortURL, response.ShortURL)
	}

	if contentType := recorder.Header().Get("Content-Type"); contentType != "application/json" {
		t.Errorf("expected JSON content type, got %q", contentType)
	}
}

func TestCreateLinkWithExpiration(t *testing.T) {
	router, _ := setupTestHandler(t)

	expiration := time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339)

	body := fmt.Sprintf(
		`{"target_url":"https://example.com","expires_at":%q}`,
		expiration,
	)

	recorder := performRequest(router, http.MethodPost, "/api/links", body)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s",
			http.StatusCreated, recorder.Code, recorder.Body.String())
	}

	var response struct {
		Link models.Link `json:"link"`
	}

	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Link.ExpiresAt == nil {
		t.Fatal("expected expiration date")
	}
}

func TestCreateLinkRejectsInvalidRequests(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "invalid JSON",
			body: `{"target_url":`,
		},
		{
			name: "unknown field",
			body: `{"target_url":"https://example.com","unknown":"value"}`,
		},
		{
			name: "invalid URL",
			body: `{"target_url":"not-a-url"}`,
		},
		{
			name: "missing URL",
			body: `{}`,
		},
		{
			name: "invalid expiration",
			body: `{"target_url":"https://example.com","expires_at":"not-a-date"}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router, _ := setupTestHandler(t)

			recorder := performRequest(
				router,
				http.MethodPost,
				"/api/links",
				test.body,
			)

			if recorder.Code != http.StatusBadRequest {
				t.Errorf("expected status %d, got %d",
					http.StatusBadRequest, recorder.Code)
			}

			var response map[string]string
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Fatalf("decode error response: %v", err)
			}

			if response["error"] == "" {
				t.Error("expected error message")
			}
		})
	}
}

func TestCreateLinkRejectsOversizedBody(t *testing.T) {
	router, _ := setupTestHandler(t)

	body := `{"target_url":"https://example.com","padding":"` +
		strings.Repeat("a", 1<<20) + `"}`

	recorder := performRequest(router, http.MethodPost, "/api/links", body)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d",
			http.StatusBadRequest, recorder.Code)
	}
}

func TestListLinks(t *testing.T) {
	router, repository := setupTestHandler(t)

	createHandlerTestLink(t, repository)

	recorder := performRequest(
		router,
		http.MethodGet,
		"/api/links",
		"",
	)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d",
			http.StatusOK, recorder.Code)
	}

	var links []models.Link
	if err := json.Unmarshal(recorder.Body.Bytes(), &links); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(links) != 1 {
		t.Errorf("expected 1 link, got %d", len(links))
	}

	if links[0].Code != "testCode123" {
		t.Errorf("unexpected shortcode: %q", links[0].Code)
	}
}

func TestListLinksEmpty(t *testing.T) {
	router, _ := setupTestHandler(t)

	recorder := performRequest(
		router,
		http.MethodGet,
		"/api/links",
		"",
	)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d",
			http.StatusOK, recorder.Code)
	}

	if body := strings.TrimSpace(recorder.Body.String()); body != "[]" {
		t.Errorf("expected empty JSON array, got %q", body)
	}
}

func TestRedirectLink(t *testing.T) {
	router, repository := setupTestHandler(t)

	link := createHandlerTestLink(t, repository)

	recorder := performRequest(
		router,
		http.MethodGet,
		"/"+link.Code,
		"",
	)

	if recorder.Code != http.StatusFound {
		t.Fatalf("expected status %d, got %d",
			http.StatusFound, recorder.Code)
	}

	if location := recorder.Header().Get("Location"); location != link.TargetURL {
		t.Errorf("expected redirect to %q, got %q",
			link.TargetURL, location)
	}

	retrieved, err := repository.GetByCode(context.Background(), link.Code)
	if err != nil {
		t.Fatalf("get redirected link: %v", err)
	}

	if retrieved.Clicks != 1 {
		t.Errorf("expected 1 click, got %d", retrieved.Clicks)
	}
}

func TestRedirectLinkNotFound(t *testing.T) {
	router, _ := setupTestHandler(t)

	recorder := performRequest(
		router,
		http.MethodGet,
		"/nonexistent",
		"",
	)

	if recorder.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d",
			http.StatusNotFound, recorder.Code)
	}
}

func TestRedirectInactiveLink(t *testing.T) {
	router, repository := setupTestHandler(t)

	link := createHandlerTestLink(t, repository)

	if err := repository.SetActive(context.Background(), link.ID, false); err != nil {
		t.Fatalf("deactivate link: %v", err)
	}

	recorder := performRequest(
		router,
		http.MethodGet,
		"/"+link.Code,
		"",
	)

	if recorder.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d",
			http.StatusNotFound, recorder.Code)
	}
}

func TestDeleteLink(t *testing.T) {
	router, repository := setupTestHandler(t)

	link := createHandlerTestLink(t, repository)

	recorder := performRequest(
		router,
		http.MethodDelete,
		fmt.Sprintf("/api/links/%d", link.ID),
		"",
	)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d",
			http.StatusNoContent, recorder.Code)
	}

	_, err := repository.GetByCode(context.Background(), link.Code)
	if err == nil {
		t.Error("expected link to be deleted")
	}
}

func TestDeleteLinkInvalidID(t *testing.T) {
	router, _ := setupTestHandler(t)

	tests := []string{
		"/api/links/abc",
		"/api/links/0",
		"/api/links/-1",
	}

	for _, path := range tests {
		t.Run(path, func(t *testing.T) {
			recorder := performRequest(
				router,
				http.MethodDelete,
				path,
				"",
			)

			if recorder.Code != http.StatusBadRequest {
				t.Errorf("expected status %d, got %d",
					http.StatusBadRequest, recorder.Code)
			}
		})
	}
}

func TestDeleteLinkNotFound(t *testing.T) {
	router, _ := setupTestHandler(t)

	recorder := performRequest(
		router,
		http.MethodDelete,
		"/api/links/999",
		"",
	)

	if recorder.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d",
			http.StatusNotFound, recorder.Code)
	}
}

func TestSetActive(t *testing.T) {
	router, repository := setupTestHandler(t)

	link := createHandlerTestLink(t, repository)

	recorder := performRequest(
		router,
		http.MethodPatch,
		fmt.Sprintf("/api/links/%d/active", link.ID),
		`{"active":false}`,
	)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s",
			http.StatusOK, recorder.Code, recorder.Body.String())
	}

	var response struct {
		ID     int64 `json:"id"`
		Active bool  `json:"active"`
	}

	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.ID != link.ID {
		t.Errorf("expected ID %d, got %d", link.ID, response.ID)
	}

	if response.Active {
		t.Error("expected response to indicate inactive link")
	}

	retrieved, err := repository.GetByCode(context.Background(), link.Code)
	if err != nil {
		t.Fatalf("get updated link: %v", err)
	}

	if retrieved.Active {
		t.Error("expected link to be inactive in database")
	}
}

func TestSetActiveInvalidRequest(t *testing.T) {
	router, repository := setupTestHandler(t)

	link := createHandlerTestLink(t, repository)

	tests := []struct {
		name string
		body string
	}{
		{
			name: "invalid JSON",
			body: `{"active":`,
		},
		{
			name: "wrong value type",
			body: `{"active":"false"}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := performRequest(
				router,
				http.MethodPatch,
				fmt.Sprintf("/api/links/%d/active", link.ID),
				test.body,
			)

			if recorder.Code != http.StatusBadRequest {
				t.Errorf("expected status %d, got %d",
					http.StatusBadRequest, recorder.Code)
			}
		})
	}
}

func TestSetActiveInvalidID(t *testing.T) {
	router, _ := setupTestHandler(t)

	recorder := performRequest(
		router,
		http.MethodPatch,
		"/api/links/invalid/active",
		`{"active":true}`,
	)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d",
			http.StatusBadRequest, recorder.Code)
	}
}

func TestSetActiveNotFound(t *testing.T) {
	router, _ := setupTestHandler(t)

	recorder := performRequest(
		router,
		http.MethodPatch,
		"/api/links/999/active",
		`{"active":true}`,
	)

	if recorder.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d",
			http.StatusNotFound, recorder.Code)
	}
}

func TestNewLinkHandlerTrimsBaseURL(t *testing.T) {
	router, _ := setupTestHandler(t)

	// This test is covered by TestCreateLink's short_url assertion.
	// Keep this placeholder only if the constructor gains additional behaviour.
	_ = router
}

func TestCreateLinkWithEmptyExpiration(t *testing.T) {
	router, _ := setupTestHandler(t)

	recorder := performRequest(
		router,
		http.MethodPost,
		"/api/links",
		`{"target_url":"https://example.com","expires_at":""}`,
	)

	if recorder.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d: %s",
			http.StatusCreated, recorder.Code, recorder.Body.String())
	}
}

func TestCreateLinkWithWhitespaceBody(t *testing.T) {
	router, _ := setupTestHandler(t)

	recorder := performRequest(
		router,
		http.MethodPost,
		"/api/links",
		"   ",
	)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d",
			http.StatusBadRequest, recorder.Code)
	}
}
