package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/unowned-22/api/internal/domain/token"
	"github.com/unowned-22/api/internal/errs"
)

// RefreshTokenRepository is the PostgreSQL implementation of token.RefreshTokenRepository.
type RefreshTokenRepository struct {
	db *pgxpool.Pool
}

// NewRefreshTokenRepository creates a new PostgreSQL implementation of RefreshTokenRepository.
func NewRefreshTokenRepository(db *pgxpool.Pool) *RefreshTokenRepository {
	return &RefreshTokenRepository{db: db}
}

// CreateRefreshToken inserts a new refresh token into the database.
func (r *RefreshTokenRepository) CreateRefreshToken(ctx context.Context, t *token.RefreshToken) error {
	query := `
		INSERT INTO refresh_tokens (user_id, token, expires_at, status, created_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`
	err := r.db.QueryRow(ctx, query, t.UserID, t.Token, t.ExpiresAt, t.Status, t.CreatedAt).Scan(&t.ID)
	if err != nil {
		return fmt.Errorf("failed to create refresh token in db: %w", err)
	}
	return nil
}

// GetByToken retrieves a refresh token by its token string value.
func (r *RefreshTokenRepository) GetByToken(ctx context.Context, tokenStr string) (*token.RefreshToken, error) {
	query := `
		SELECT id, user_id, token, expires_at, status, created_at
		FROM refresh_tokens
		WHERE token = $1
	`
	var t token.RefreshToken
	err := r.db.QueryRow(ctx, query, tokenStr).
		Scan(&t.ID, &t.UserID, &t.Token, &t.ExpiresAt, &t.Status, &t.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.ErrRefreshTokenNotFound
		}
		return nil, fmt.Errorf("failed to get refresh token by token from db: %w", err)
	}
	return &t, nil
}

// RevokeRefreshToken marks a refresh token as revoked.
func (r *RefreshTokenRepository) RevokeRefreshToken(ctx context.Context, tokenStr string) error {
	query := `
		UPDATE refresh_tokens
		SET status = $1
		WHERE token = $2
	`
	res, err := r.db.Exec(ctx, query, token.RefreshTokenStatusRevoked, tokenStr)
	if err != nil {
		return fmt.Errorf("failed to revoke refresh token in db: %w", err)
	}
	if res.RowsAffected() == 0 {
		return errs.ErrRefreshTokenNotFound
	}
	return nil
}

// DeleteExpired deletes all expired refresh tokens from the database.
func (r *RefreshTokenRepository) DeleteExpired(ctx context.Context) error {
	query := `DELETE FROM refresh_tokens WHERE expires_at < NOW()`
	_, err := r.db.Exec(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to delete expired refresh tokens: %w", err)
	}
	return nil
}

// RevokeAllByUserID revokes all refresh tokens for a given user.
func (r *RefreshTokenRepository) RevokeAllByUserID(ctx context.Context, userID int64) error {
	query := `
		UPDATE refresh_tokens
		SET status = $1
		WHERE user_id = $2
	`
	_, err := r.db.Exec(ctx, query, token.RefreshTokenStatusRevoked, userID)
	if err != nil {
		return fmt.Errorf("failed to revoke refresh tokens by user id in db: %w", err)
	}
	return nil
}

// Compile-time check that RefreshTokenRepository satisfies the domain contract.
var _ token.RefreshTokenRepository = (*RefreshTokenRepository)(nil)
