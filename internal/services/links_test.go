package services

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lilleole/shrinkr/internal/database"

	_ "modernc.org/sqlite"
)

func setupTestService(t *testing.T, defaultExpirationDays int) *LinkService {
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

	t.Cleanup(func() {
		db.Close()
	})

	repository := database.NewLinkRepository(db)

	return NewLinkService(repository, defaultExpirationDays)
}

func TestCreate(t *testing.T) {
	service := setupTestService(t, 7)
	ctx := context.Background()

	link, err := service.Create(ctx, "  https://example.com  ")
	if err != nil {
		t.Fatalf("create link: %v", err)
	}

	if link.Code == "" {
		t.Error("expected generated shortcode")
	}

	if len(link.Code) != shortCodeLength {
		t.Errorf("expected shortcode length %d, got %d",
			shortCodeLength, len(link.Code))
	}

	if link.TargetURL != "https://example.com" {
		t.Errorf("unexpected target URL: %q", link.TargetURL)
	}

	if !link.Active {
		t.Error("expected new link to be active")
	}

	if link.Clicks != 0 {
		t.Errorf("expected 0 clicks, got %d", link.Clicks)
	}

	if link.ExpiresAt == nil {
		t.Fatal("expected default expiration")
	}

	if !link.ExpiresAt.After(time.Now()) {
		t.Error("expected expiration to be in the future")
	}

	if link.CreatedAt.IsZero() {
		t.Error("expected creation timestamp")
	}
}

func TestCreateWithoutDefaultExpiration(t *testing.T) {
	service := setupTestService(t, 0)

	link, err := service.Create(context.Background(), "https://example.com")
	if err != nil {
		t.Fatalf("create link: %v", err)
	}

	if link.ExpiresAt != nil {
		t.Errorf("expected no expiration, got %v", link.ExpiresAt)
	}
}

func TestCreateWithCustomExpiration(t *testing.T) {
	service := setupTestService(t, 7)

	expiration := time.Now().Add(24 * time.Hour)

	link, err := service.CreateWithExpiration(
		context.Background(),
		"https://example.com",
		&expiration,
	)
	if err != nil {
		t.Fatalf("create link: %v", err)
	}

	if link.ExpiresAt == nil {
		t.Fatal("expected custom expiration")
	}

	if !link.ExpiresAt.Equal(expiration) {
		t.Errorf("expected expiration %v, got %v",
			expiration, link.ExpiresAt)
	}
}

func TestCreateRejectsInvalidURLs(t *testing.T) {
	service := setupTestService(t, 0)

	tests := []struct {
		name string
		url  string
	}{
		{"empty URL", ""},
		{"whitespace", "   "},
		{"missing scheme", "example.com"},
		{"relative URL", "/path/to/page"},
		{"FTP scheme", "ftp://example.com"},
		{"JavaScript scheme", "javascript:alert(1)"},
		{"missing hostname", "https:///path"},
		{"URL with username", "https://user:password@example.com"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := service.Create(context.Background(), test.url)

			if !errors.Is(err, ErrInvalidURL) {
				t.Errorf("expected ErrInvalidURL, got %v", err)
			}
		})
	}
}

func TestCreateAcceptsValidURLs(t *testing.T) {
	service := setupTestService(t, 0)

	tests := []string{
		"https://example.com",
		"http://example.com",
		"https://example.com/path",
		"https://example.com/path?query=value",
		"https://localhost:8080",
	}

	for _, targetURL := range tests {
		t.Run(targetURL, func(t *testing.T) {
			_, err := service.Create(context.Background(), targetURL)
			if err != nil {
				t.Errorf("expected valid URL, got error: %v", err)
			}
		})
	}
}

func TestCreateRejectsPastExpiration(t *testing.T) {
	service := setupTestService(t, 0)

	expiration := time.Now().Add(-time.Hour)

	_, err := service.CreateWithExpiration(
		context.Background(),
		"https://example.com",
		&expiration,
	)

	if err == nil {
		t.Fatal("expected error for past expiration")
	}

	if err.Error() != "expiration date must be in the future" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGet(t *testing.T) {
	service := setupTestService(t, 0)
	ctx := context.Background()

	created, err := service.Create(ctx, "https://example.com")
	if err != nil {
		t.Fatalf("create link: %v", err)
	}

	retrieved, err := service.Get(ctx, created.Code)
	if err != nil {
		t.Fatalf("get link: %v", err)
	}

	if retrieved.Code != created.Code {
		t.Errorf("expected code %q, got %q",
			created.Code, retrieved.Code)
	}

	if retrieved.TargetURL != created.TargetURL {
		t.Errorf("expected URL %q, got %q",
			created.TargetURL, retrieved.TargetURL)
	}
}

func TestGetInactiveLink(t *testing.T) {
	service := setupTestService(t, 0)
	ctx := context.Background()

	link, err := service.Create(ctx, "https://example.com")
	if err != nil {
		t.Fatalf("create link: %v", err)
	}

	if err := service.SetActive(ctx, link.ID, false); err != nil {
		t.Fatalf("deactivate link: %v", err)
	}

	_, err = service.Get(ctx, link.Code)

	if !errors.Is(err, database.ErrLinkNotFound) {
		t.Errorf("expected ErrLinkNotFound, got %v", err)
	}
}

func TestGetUnknownLink(t *testing.T) {
	service := setupTestService(t, 0)

	_, err := service.Get(context.Background(), "doesnotexist")

	if !errors.Is(err, database.ErrLinkNotFound) {
		t.Errorf("expected ErrLinkNotFound, got %v", err)
	}
}

func TestList(t *testing.T) {
	service := setupTestService(t, 0)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		_, err := service.Create(ctx, "https://example.com")
		if err != nil {
			t.Fatalf("create link: %v", err)
		}
	}

	links, err := service.List(ctx)
	if err != nil {
		t.Fatalf("list links: %v", err)
	}

	if len(links) != 3 {
		t.Errorf("expected 3 links, got %d", len(links))
	}
}

func TestDelete(t *testing.T) {
	service := setupTestService(t, 0)
	ctx := context.Background()

	link, err := service.Create(ctx, "https://example.com")
	if err != nil {
		t.Fatalf("create link: %v", err)
	}

	if err := service.Delete(ctx, link.ID); err != nil {
		t.Fatalf("delete link: %v", err)
	}

	_, err = service.Get(ctx, link.Code)
	if !errors.Is(err, database.ErrLinkNotFound) {
		t.Errorf("expected ErrLinkNotFound after deletion, got %v", err)
	}
}

func TestSetActive(t *testing.T) {
	service := setupTestService(t, 0)
	ctx := context.Background()

	link, err := service.Create(ctx, "https://example.com")
	if err != nil {
		t.Fatalf("create link: %v", err)
	}

	if err := service.SetActive(ctx, link.ID, false); err != nil {
		t.Fatalf("deactivate link: %v", err)
	}

	links, err := service.List(ctx)
	if err != nil {
		t.Fatalf("list links: %v", err)
	}

	if len(links) != 1 || links[0].Active {
		t.Error("expected link to be inactive")
	}

	if err := service.SetActive(ctx, link.ID, true); err != nil {
		t.Fatalf("activate link: %v", err)
	}

	retrieved, err := service.Get(ctx, link.Code)
	if err != nil {
		t.Fatalf("get reactivated link: %v", err)
	}

	if !retrieved.Active {
		t.Error("expected link to be active")
	}
}

func TestRecordClick(t *testing.T) {
	service := setupTestService(t, 0)
	ctx := context.Background()

	link, err := service.Create(ctx, "https://example.com")
	if err != nil {
		t.Fatalf("create link: %v", err)
	}

	for i := 0; i < 3; i++ {
		if err := service.RecordClick(ctx, link.ID); err != nil {
			t.Fatalf("record click: %v", err)
		}
	}

	links, err := service.List(ctx)
	if err != nil {
		t.Fatalf("list links: %v", err)
	}

	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}

	if links[0].Clicks != 3 {
		t.Errorf("expected 3 clicks, got %d", links[0].Clicks)
	}
}

func TestGenerateShortCode(t *testing.T) {
	code, err := generateShortCode(shortCodeLength)
	if err != nil {
		t.Fatalf("generate shortcode: %v", err)
	}

	if len(code) != shortCodeLength {
		t.Errorf("expected length %d, got %d",
			shortCodeLength, len(code))
	}

	for _, character := range code {
		if !strings.ContainsRune(charset, character) {
			t.Errorf("unexpected character in shortcode: %q", character)
		}
	}
}
