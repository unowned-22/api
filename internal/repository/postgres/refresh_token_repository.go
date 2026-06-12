package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	domain "github.com/unowned-22/api/internal/domain/user"
	"github.com/unowned-22/api/internal/errs"
)

type RefreshTokenRepository struct {
	db *pgxpool.Pool
}

// NewRefreshTokenRepository creates a new PostgreSQL implementation of RefreshTokenRepository
func NewRefreshTokenRepository(db *pgxpool.Pool) *RefreshTokenRepository {
	return &RefreshTokenRepository{db: db}
}

// Create inserts a new refresh token into the database
func (r *RefreshTokenRepository) Create(ctx context.Context, t *domain.RefreshToken) error {
	query := `
		INSERT INTO refresh_tokens (user_id, token, expires_at, created_at, revoked)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`
	err := r.db.QueryRow(ctx, query, t.UserID, t.Token, t.ExpiresAt, t.CreatedAt, t.Revoked).Scan(&t.ID)
	if err != nil {
		return fmt.Errorf("failed to create refresh token in db: %w", err)
	}
	return nil
}

// GetByToken retrieves a refresh token by its token string value
func (r *RefreshTokenRepository) GetByToken(ctx context.Context, token string) (*domain.RefreshToken, error) {
	query := `
		SELECT id, user_id, token, expires_at, revoked, created_at
		FROM refresh_tokens
		WHERE token = $1
	`
	var t domain.RefreshToken
	err := r.db.QueryRow(ctx, query, token).Scan(&t.ID, &t.UserID, &t.Token, &t.ExpiresAt, &t.Revoked, &t.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.ErrRefreshTokenNotFound
		}
		return nil, fmt.Errorf("failed to get refresh token by token from db: %w", err)
	}
	return &t, nil
}

// Revoke marks a refresh token as revoked
func (r *RefreshTokenRepository) Revoke(ctx context.Context, token string) error {
	query := `
		UPDATE refresh_tokens
		SET revoked = TRUE
		WHERE token = $1
	`
	res, err := r.db.Exec(ctx, query, token)
	if err != nil {
		return fmt.Errorf("failed to revoke refresh token in db: %w", err)
	}
	if res.RowsAffected() == 0 {
		return errs.ErrRefreshTokenNotFound
	}
	return nil
}

// DeleteExpired deletes all expired refresh tokens from the database
func (r *RefreshTokenRepository) DeleteExpired(ctx context.Context) error {
	query := `
		DELETE FROM refresh_tokens
		WHERE expires_at < NOW()
	`
	_, err := r.db.Exec(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to delete expired refresh tokens: %w", err)
	}
	return nil
}

// Ensure RefreshTokenRepository implements domain.RefreshTokenRepository
var _ domain.RefreshTokenRepository = (*RefreshTokenRepository)(nil)
