package services

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"strings"
	"time"

	"github.com/lilleole/shrinkr/internal/database"
	"github.com/lilleole/shrinkr/internal/models"
)

const (
	shortCodeLength = 16
	maxRetries      = 5
	charset         = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
)

var (
	ErrInvalidURL = errors.New("invalid URL")
	ErrCodeExists = errors.New("could not generate a unique shortcode")
)

type LinkService struct {
	repository            *database.LinkRepository
	defaultExpirationDays int
}

func NewLinkService(
	repository *database.LinkRepository,
	defaultExpirationDays int,
) *LinkService {
	return &LinkService{
		repository:            repository,
		defaultExpirationDays: defaultExpirationDays,
	}
}

// Create creates a new shortened link using the configured default expiration.
func (s *LinkService) Create(
	ctx context.Context,
	targetURL string,
) (*models.Link, error) {
	return s.CreateWithExpiration(ctx, targetURL, nil)
}

// CreateWithExpiration creates a shortened link with an optional custom expiration.
// A nil expiration uses the configured default.
func (s *LinkService) CreateWithExpiration(
	ctx context.Context,
	targetURL string,
	expiresAt *time.Time,
) (*models.Link, error) {
	targetURL = strings.TrimSpace(targetURL)

	if err := validateURL(targetURL); err != nil {
		return nil, err
	}

	// Apply default expiration if no custom expiration was provided.
	if expiresAt == nil && s.defaultExpirationDays > 0 {
		expiration := time.Now().UTC().AddDate(
			0,
			0,
			s.defaultExpirationDays,
		)
		expiresAt = &expiration
	}

	// Reject expiration dates in the past.
	if expiresAt != nil && !expiresAt.After(time.Now()) {
		return nil, errors.New("expiration date must be in the future")
	}

	for range maxRetries {
		code, err := generateShortCode(shortCodeLength)
		if err != nil {
			return nil, fmt.Errorf("generate shortcode: %w", err)
		}

		link := &models.Link{
			Code:      code,
			TargetURL: targetURL,
			CreatedAt: time.Now().UTC(),
			ExpiresAt: expiresAt,
			Clicks:    0,
			Active:    true,
		}

		// Check whether the generated code already exists.
		existing, err := s.repository.GetByCode(ctx, code)

		if err != nil && !errors.Is(err, database.ErrLinkNotFound) {
			return nil, fmt.Errorf("check shortcode: %w", err)
		}

		if existing != nil {
			continue
		}

		if err := s.repository.Create(ctx, link); err != nil {
			// Check whether another request created the same code.
			// If so, generate another one.
			existing, lookupErr := s.repository.GetByCode(ctx, code)

			if lookupErr == nil && existing != nil {
				continue
			}

			return nil, fmt.Errorf("create link: %w", err)
		}

		return link, nil
	}

	return nil, ErrCodeExists
}

// Get retrieves a link by its shortcode.
func (s *LinkService) Get(
	ctx context.Context,
	code string,
) (*models.Link, error) {
	link, err := s.repository.GetByCode(ctx, code)
	if err != nil {
		return nil, err
	}

	if !link.Active {
		return nil, database.ErrLinkNotFound
	}

	if link.ExpiresAt != nil && !link.ExpiresAt.After(time.Now()) {
		return nil, database.ErrLinkNotFound
	}

	return link, nil
}

// List retrieves all links.
func (s *LinkService) List(
	ctx context.Context,
) ([]models.Link, error) {
	return s.repository.GetAll(ctx)
}

// Delete removes a link.
func (s *LinkService) Delete(
	ctx context.Context,
	id int64,
) error {
	return s.repository.Delete(ctx, id)
}

// SetActive enables or disables a link.
func (s *LinkService) SetActive(
	ctx context.Context,
	id int64,
	active bool,
) error {
	return s.repository.SetActive(ctx, id, active)
}

// RecordClick increments the click counter.
func (s *LinkService) RecordClick(
	ctx context.Context,
	id int64,
) error {
	return s.repository.IncrementClicks(ctx, id)
}

// validateURL ensures the destination is a valid HTTP or HTTPS URL.
func validateURL(rawURL string) error {
	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return ErrInvalidURL
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return ErrInvalidURL
	}

	if parsed.Hostname() == "" || parsed.User != nil {
		return ErrInvalidURL
	}

	return nil
}

// generateShortCode creates a cryptographically random shortcode.
func generateShortCode(length int) (string, error) {
	var result strings.Builder
	result.Grow(length)

	for range length {
		index, err := rand.Int(
			rand.Reader,
			big.NewInt(int64(len(charset))),
		)
		if err != nil {
			return "", err
		}

		result.WriteByte(charset[index.Int64()])
	}

	return result.String(), nil
}
