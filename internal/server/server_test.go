package server

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lilleole/shrinkr/internal/database"
	"github.com/lilleole/shrinkr/internal/services"

	_ "modernc.org/sqlite"
)

func setupTestServer(t *testing.T) http.Handler {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}

	db.SetMaxOpenConns(1)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := database.Migrate(ctx, db); err != nil {
		db.Close()
		t.Fatalf("run migrations: %v", err)
	}

	repository := database.NewLinkRepository(db)
	linkService := services.NewLinkService(repository, 30)

	t.Cleanup(func() {
		db.Close()
	})

	return New(db, linkService, "http://localhost:8080")
}

func TestRootEndpoint(t *testing.T) {
	handler := setupTestServer(t)

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d",
			http.StatusOK, recorder.Code)
	}

	if body := recorder.Body.String(); body != "Shrinkr is running!" {
		t.Errorf("unexpected response body: %q", body)
	}
}

func TestHealthEndpoint(t *testing.T) {
	handler := setupTestServer(t)

	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d",
			http.StatusOK, recorder.Code)
	}

	if body := recorder.Body.String(); body != `{"status":"healthy"}` {
		t.Errorf("unexpected response body: %q", body)
	}
}
