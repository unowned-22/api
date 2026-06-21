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

// Create inserts a new refresh token and sets its ID via RETURNING.
func (r *RefreshTokenRepository) Create(ctx context.Context, t *token.RefreshToken) error {
	query := `
		INSERT INTO refresh_tokens (session_id, user_id, token_hash, parent_token_id, status, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`
	err := r.db.QueryRow(ctx, query,
		t.SessionID, t.UserID, t.TokenHash, t.ParentTokenID, t.Status, t.CreatedAt, t.ExpiresAt,
	).Scan(&t.ID)
	if err != nil {
		return fmt.Errorf("failed to create refresh token: %w", err)
	}
	return nil
}

// GetByHash retrieves a token by its pre-hashed value. The caller must hash the raw token first.
func (r *RefreshTokenRepository) GetByHash(ctx context.Context, tokenHash string) (*token.RefreshToken, error) {
	query := `
		SELECT id, session_id, user_id, token_hash, parent_token_id, replaced_by_token_id, status, created_at, expires_at
		FROM refresh_tokens
		WHERE token_hash = $1
	`
	var t token.RefreshToken
	err := r.db.QueryRow(ctx, query, tokenHash).Scan(
		&t.ID, &t.SessionID, &t.UserID, &t.TokenHash,
		&t.ParentTokenID, &t.ReplacedByTokenID,
		&t.Status, &t.CreatedAt, &t.ExpiresAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.ErrRefreshTokenNotFound
		}
		return nil, fmt.Errorf("failed to get refresh token by hash: %w", err)
	}
	return &t, nil
}

// Rotate atomically revokes the old token and inserts the new one in a single transaction.
// It sets replaced_by_token_id on the old token and parent_token_id on the new one.
func (r *RefreshTokenRepository) Rotate(ctx context.Context, oldTokenID int64, newToken *token.RefreshToken) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin rotate transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Insert new token
	insertQuery := `
		INSERT INTO refresh_tokens (session_id, user_id, token_hash, parent_token_id, status, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`
	err = tx.QueryRow(ctx, insertQuery,
		newToken.SessionID, newToken.UserID, newToken.TokenHash,
		newToken.ParentTokenID, newToken.Status, newToken.CreatedAt, newToken.ExpiresAt,
	).Scan(&newToken.ID)
	if err != nil {
		return fmt.Errorf("failed to insert new token during rotate: %w", err)
	}

	// Revoke old token and set replaced_by_token_id
	updateQuery := `
		UPDATE refresh_tokens
		SET status = $1, replaced_by_token_id = $2
		WHERE id = $3
	`
	_, err = tx.Exec(ctx, updateQuery, token.RefreshTokenStatusRevoked, newToken.ID, oldTokenID)
	if err != nil {
		return fmt.Errorf("failed to revoke old token during rotate: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit rotate transaction: %w", err)
	}
	return nil
}

// Revoke marks a single token as revoked by ID.
func (r *RefreshTokenRepository) Revoke(ctx context.Context, tokenID int64) error {
	query := `UPDATE refresh_tokens SET status = $1 WHERE id = $2`
	_, err := r.db.Exec(ctx, query, token.RefreshTokenStatusRevoked, tokenID)
	if err != nil {
		return fmt.Errorf("failed to revoke refresh token: %w", err)
	}
	return nil
}

// RevokeSessionTokens revokes all active tokens belonging to a session.
func (r *RefreshTokenRepository) RevokeSessionTokens(ctx context.Context, sessionID int64) error {
	query := `
		UPDATE refresh_tokens
		SET status = $1
		WHERE session_id = $2 AND status = 'active'
	`
	_, err := r.db.Exec(ctx, query, token.RefreshTokenStatusRevoked, sessionID)
	if err != nil {
		return fmt.Errorf("failed to revoke session tokens: %w", err)
	}
	return nil
}

// RevokeAllByUserID revokes all tokens for a user.
func (r *RefreshTokenRepository) RevokeAllByUserID(ctx context.Context, userID int64) error {
	query := `UPDATE refresh_tokens SET status = $1 WHERE user_id = $2`
	_, err := r.db.Exec(ctx, query, token.RefreshTokenStatusRevoked, userID)
	if err != nil {
		return fmt.Errorf("failed to revoke all refresh tokens for user: %w", err)
	}
	return nil
}

// GetTokenChain returns all tokens for a session ordered by created_at ASC.
func (r *RefreshTokenRepository) GetTokenChain(ctx context.Context, sessionID int64) ([]*token.RefreshToken, error) {
	query := `
		SELECT id, session_id, user_id, token_hash, parent_token_id, replaced_by_token_id, status, created_at, expires_at
		FROM refresh_tokens
		WHERE session_id = $1
		ORDER BY created_at ASC
	`
	rows, err := r.db.Query(ctx, query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get token chain: %w", err)
	}
	defer rows.Close()

	var tokens []*token.RefreshToken
	for rows.Next() {
		var t token.RefreshToken
		if err := rows.Scan(
			&t.ID, &t.SessionID, &t.UserID, &t.TokenHash,
			&t.ParentTokenID, &t.ReplacedByTokenID,
			&t.Status, &t.CreatedAt, &t.ExpiresAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan token row: %w", err)
		}
		tokens = append(tokens, &t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate token rows: %w", err)
	}
	return tokens, nil
}

// DeleteExpired removes all expired refresh tokens from the database.
func (r *RefreshTokenRepository) DeleteExpired(ctx context.Context) error {
	query := `DELETE FROM refresh_tokens WHERE expires_at < NOW()`
	_, err := r.db.Exec(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to delete expired refresh tokens: %w", err)
	}
	return nil
}

// Compile-time check that RefreshTokenRepository satisfies the domain contract.
var _ token.RefreshTokenRepository = (*RefreshTokenRepository)(nil)
