package database

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/lilleole/shrinkr/internal/models"
)

func setupTestRepository(t *testing.T) *LinkRepository {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	db, err := Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}

	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close test database: %v", err)
		}
	})

	return NewLinkRepository(db)
}

func createTestLink(t *testing.T, repository *LinkRepository) *models.Link {
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

func TestCreate(t *testing.T) {
	repository := setupTestRepository(t)

	link := &models.Link{
		Code:      "testCode123",
		TargetURL: "https://example.com",
		Active:    true,
	}

	err := repository.Create(context.Background(), link)
	if err != nil {
		t.Fatalf("create link: %v", err)
	}

	if link.ID <= 0 {
		t.Errorf("expected generated ID, got %d", link.ID)
	}

	retrieved, err := repository.GetByCode(context.Background(), link.Code)
	if err != nil {
		t.Fatalf("get created link: %v", err)
	}

	if retrieved.TargetURL != link.TargetURL {
		t.Errorf("expected URL %q, got %q",
			link.TargetURL, retrieved.TargetURL)
	}

	if !retrieved.Active {
		t.Error("expected link to be active")
	}
}

func TestCreateWithExpiration(t *testing.T) {
	repository := setupTestRepository(t)

	expiration := time.Now().UTC().Add(24 * time.Hour)

	link := &models.Link{
		Code:      "expiringLink",
		TargetURL: "https://example.com",
		ExpiresAt: &expiration,
		Active:    true,
	}

	if err := repository.Create(context.Background(), link); err != nil {
		t.Fatalf("create link: %v", err)
	}

	retrieved, err := repository.GetByCode(context.Background(), link.Code)
	if err != nil {
		t.Fatalf("get link: %v", err)
	}

	if retrieved.ExpiresAt == nil {
		t.Fatal("expected expiration date")
	}

	if !retrieved.ExpiresAt.Equal(expiration) {
		t.Errorf("expected expiration %v, got %v",
			expiration, *retrieved.ExpiresAt)
	}
}

func TestCreateRejectsDuplicateCode(t *testing.T) {
	repository := setupTestRepository(t)
	ctx := context.Background()

	createTestLink(t, repository)

	duplicate := &models.Link{
		Code:      "testCode123",
		TargetURL: "https://another-example.com",
		Active:    true,
	}

	err := repository.Create(ctx, duplicate)
	if err == nil {
		t.Fatal("expected error when creating duplicate shortcode")
	}
}

func TestGetByCode(t *testing.T) {
	repository := setupTestRepository(t)

	created := createTestLink(t, repository)

	retrieved, err := repository.GetByCode(
		context.Background(),
		created.Code,
	)
	if err != nil {
		t.Fatalf("get link: %v", err)
	}

	if retrieved.ID != created.ID {
		t.Errorf("expected ID %d, got %d", created.ID, retrieved.ID)
	}

	if retrieved.Code != created.Code {
		t.Errorf("expected code %q, got %q",
			created.Code, retrieved.Code)
	}

	if retrieved.TargetURL != created.TargetURL {
		t.Errorf("expected URL %q, got %q",
			created.TargetURL, retrieved.TargetURL)
	}

	if retrieved.Clicks != 0 {
		t.Errorf("expected 0 clicks, got %d", retrieved.Clicks)
	}

	if !retrieved.Active {
		t.Error("expected active link")
	}

	if retrieved.CreatedAt.IsZero() {
		t.Error("expected creation timestamp")
	}

	if retrieved.ExpiresAt != nil {
		t.Error("expected no expiration")
	}
}

func TestGetByCodeNotFound(t *testing.T) {
	repository := setupTestRepository(t)

	_, err := repository.GetByCode(
		context.Background(),
		"nonexistent",
	)

	if !errors.Is(err, ErrLinkNotFound) {
		t.Errorf("expected ErrLinkNotFound, got %v", err)
	}
}

func TestGetAll(t *testing.T) {
	repository := setupTestRepository(t)

	first := createTestLink(t, repository)

	second := &models.Link{
		Code:      "anotherCode",
		TargetURL: "https://another-example.com",
		Active:    false,
	}

	if err := repository.Create(context.Background(), second); err != nil {
		t.Fatalf("create second link: %v", err)
	}

	links, err := repository.GetAll(context.Background())
	if err != nil {
		t.Fatalf("get all links: %v", err)
	}

	if len(links) != 2 {
		t.Fatalf("expected 2 links, got %d", len(links))
	}

	found := map[string]bool{}

	for _, link := range links {
		found[link.Code] = true
	}

	if !found[first.Code] {
		t.Errorf("expected to find link %q", first.Code)
	}

	if !found[second.Code] {
		t.Errorf("expected to find link %q", second.Code)
	}
}

func TestGetAllEmpty(t *testing.T) {
	repository := setupTestRepository(t)

	links, err := repository.GetAll(context.Background())
	if err != nil {
		t.Fatalf("get all links: %v", err)
	}

	if len(links) != 0 {
		t.Errorf("expected empty result, got %d links", len(links))
	}
}

func TestIncrementClicks(t *testing.T) {
	repository := setupTestRepository(t)
	ctx := context.Background()

	link := createTestLink(t, repository)

	for i := 0; i < 3; i++ {
		if err := repository.IncrementClicks(ctx, link.ID); err != nil {
			t.Fatalf("increment clicks: %v", err)
		}
	}

	retrieved, err := repository.GetByCode(ctx, link.Code)
	if err != nil {
		t.Fatalf("get link: %v", err)
	}

	if retrieved.Clicks != 3 {
		t.Errorf("expected 3 clicks, got %d", retrieved.Clicks)
	}
}

func TestIncrementClicksNotFound(t *testing.T) {
	repository := setupTestRepository(t)

	err := repository.IncrementClicks(context.Background(), 999)

	if !errors.Is(err, ErrLinkNotFound) {
		t.Errorf("expected ErrLinkNotFound, got %v", err)
	}
}

func TestDelete(t *testing.T) {
	repository := setupTestRepository(t)
	ctx := context.Background()

	link := createTestLink(t, repository)

	if err := repository.Delete(ctx, link.ID); err != nil {
		t.Fatalf("delete link: %v", err)
	}

	_, err := repository.GetByCode(ctx, link.Code)
	if !errors.Is(err, ErrLinkNotFound) {
		t.Errorf("expected ErrLinkNotFound after deletion, got %v", err)
	}
}

func TestDeleteNotFound(t *testing.T) {
	repository := setupTestRepository(t)

	err := repository.Delete(context.Background(), 999)

	if !errors.Is(err, ErrLinkNotFound) {
		t.Errorf("expected ErrLinkNotFound, got %v", err)
	}
}

func TestSetActive(t *testing.T) {
	repository := setupTestRepository(t)
	ctx := context.Background()

	link := createTestLink(t, repository)

	if err := repository.SetActive(ctx, link.ID, false); err != nil {
		t.Fatalf("deactivate link: %v", err)
	}

	retrieved, err := repository.GetByCode(ctx, link.Code)
	if err != nil {
		t.Fatalf("get link: %v", err)
	}

	if retrieved.Active {
		t.Error("expected link to be inactive")
	}

	if err := repository.SetActive(ctx, link.ID, true); err != nil {
		t.Fatalf("activate link: %v", err)
	}

	retrieved, err = repository.GetByCode(ctx, link.Code)
	if err != nil {
		t.Fatalf("get reactivated link: %v", err)
	}

	if !retrieved.Active {
		t.Error("expected link to be active")
	}
}

func TestSetActiveNotFound(t *testing.T) {
	repository := setupTestRepository(t)

	err := repository.SetActive(context.Background(), 999, true)

	if !errors.Is(err, ErrLinkNotFound) {
		t.Errorf("expected ErrLinkNotFound, got %v", err)
	}
}
