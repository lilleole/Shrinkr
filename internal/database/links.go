package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/lilleole/shrinkr/internal/models"
)

var ErrLinkNotFound = errors.New("link not found")

type LinkRepository struct {
	db *sql.DB
}

func NewLinkRepository(db *sql.DB) *LinkRepository {
	return &LinkRepository{db: db}
}

func (r *LinkRepository) Create(ctx context.Context, link *models.Link) error {
	query := `
        INSERT INTO links (code, target_url, expires_at, active)
        VALUES (?, ?, ?, ?)
    `

	result, err := r.db.ExecContext(
		ctx,
		query,
		link.Code,
		link.TargetURL,
		link.ExpiresAt,
		link.Active,
	)
	if err != nil {
		return err
	}

	link.ID, err = result.LastInsertId()
	if err != nil {
		return err
	}

	return nil
}

func (r *LinkRepository) GetByCode(ctx context.Context, code string) (*models.Link, error) {
	query := `
        SELECT id, code, target_url, created_at, expires_at, clicks, active
        FROM links
        WHERE code = ?
    `

	var link models.Link
	var expiresAt sql.NullTime

	err := r.db.QueryRowContext(ctx, query, code).Scan(
		&link.ID,
		&link.Code,
		&link.TargetURL,
		&link.CreatedAt,
		&expiresAt,
		&link.Clicks,
		&link.Active,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrLinkNotFound
	}
	if err != nil {
		return nil, err
	}

	if expiresAt.Valid {
		link.ExpiresAt = &expiresAt.Time
	}

	return &link, nil
}

func (r *LinkRepository) GetAll(ctx context.Context) ([]models.Link, error) {
	query := `
        SELECT id, code, target_url, created_at, expires_at, clicks, active
        FROM links
        ORDER BY created_at DESC
    `

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var links []models.Link

	for rows.Next() {
		var link models.Link
		var expiresAt sql.NullTime

		err := rows.Scan(
			&link.ID,
			&link.Code,
			&link.TargetURL,
			&link.CreatedAt,
			&expiresAt,
			&link.Clicks,
			&link.Active,
		)
		if err != nil {
			return nil, err
		}

		if expiresAt.Valid {
			link.ExpiresAt = &expiresAt.Time
		}

		links = append(links, link)
	}

	return links, rows.Err()
}

func (r *LinkRepository) IncrementClicks(ctx context.Context, id int64) error {
	query := `
        UPDATE links
        SET clicks = clicks + 1
        WHERE id = ?
    `

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if affected == 0 {
		return ErrLinkNotFound
	}

	return nil
}

func (r *LinkRepository) Delete(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(
		ctx,
		`DELETE FROM links WHERE id = ?`,
		id,
	)
	if err != nil {
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if affected == 0 {
		return ErrLinkNotFound
	}

	return nil
}

func (r *LinkRepository) SetActive(ctx context.Context, id int64, active bool) error {
	result, err := r.db.ExecContext(
		ctx,
		`UPDATE links SET active = ? WHERE id = ?`,
		active,
		id,
	)
	if err != nil {
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if affected == 0 {
		return ErrLinkNotFound
	}

	return nil
}

// Avoid unused imports if extending this file later.
var _ = time.Time{}
