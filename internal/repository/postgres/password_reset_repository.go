package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/unowned-22/api/internal/domain/passwordreset"
	"github.com/unowned-22/api/internal/errs"
)

type PasswordResetRepository struct {
	db *pgxpool.Pool
}

func NewPasswordResetRepository(db *pgxpool.Pool) *PasswordResetRepository {
	return &PasswordResetRepository{db: db}
}

func (r *PasswordResetRepository) Create(ctx context.Context, t *passwordreset.Token) error {
	query := `
		INSERT INTO password_reset_tokens (user_id, token, expires_at, created_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`
	if err := r.db.QueryRow(ctx, query, t.UserID, t.Token, t.ExpiresAt, t.CreatedAt).Scan(&t.ID); err != nil {
		return fmt.Errorf("failed to create password reset token in db: %w", err)
	}
	return nil
}

func (r *PasswordResetRepository) GetByToken(ctx context.Context, token string) (*passwordreset.Token, error) {
	query := `
		SELECT id, user_id, token, expires_at, used_at, created_at
		FROM password_reset_tokens
		WHERE token = $1
	`
	var t passwordreset.Token
	err := r.db.QueryRow(ctx, query, token).
		Scan(&t.ID, &t.UserID, &t.Token, &t.ExpiresAt, &t.UsedAt, &t.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.ErrPasswordResetTokenInvalid
		}
		return nil, fmt.Errorf("failed to get password reset token from db: %w", err)
	}
	return &t, nil
}

func (r *PasswordResetRepository) MarkUsed(ctx context.Context, token string) error {
	query := `
		UPDATE password_reset_tokens
		SET used_at = $1
		WHERE token = $2
	`
	res, err := r.db.Exec(ctx, query, time.Now(), token)
	if err != nil {
		return fmt.Errorf("failed to mark password reset token used in db: %w", err)
	}
	if res.RowsAffected() == 0 {
		return errs.ErrPasswordResetTokenInvalid
	}
	return nil
}

func (r *PasswordResetRepository) DeleteByUserID(ctx context.Context, userID int64) error {
	query := `DELETE FROM password_reset_tokens WHERE user_id = $1`
	if _, err := r.db.Exec(ctx, query, userID); err != nil {
		return fmt.Errorf("failed to delete password reset tokens by user id: %w", err)
	}
	return nil
}

var _ passwordreset.Repository = (*PasswordResetRepository)(nil)
