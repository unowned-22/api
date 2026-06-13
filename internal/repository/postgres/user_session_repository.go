package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/unowned-22/api/internal/domain/usersession"
	"github.com/unowned-22/api/internal/errs"
)

// UserSessionRepository is the PostgreSQL implementation of usersession.UserSessionRepository.
type UserSessionRepository struct {
	db *pgxpool.Pool
}

// NewUserSessionRepository creates a new PostgreSQL implementation of UserSessionRepository.
func NewUserSessionRepository(db *pgxpool.Pool) *UserSessionRepository {
	return &UserSessionRepository{db: db}
}

// Create inserts a new user session into the database.
func (r *UserSessionRepository) Create(ctx context.Context, s *usersession.UserSession) error {
	query := `
		INSERT INTO user_sessions (user_id, refresh_token_id, device_name, user_agent, ip_address, created_at, last_used_at, revoked_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id
	`
	err := r.db.QueryRow(ctx, query,
		s.UserID,
		s.RefreshTokenID,
		s.DeviceName,
		s.UserAgent,
		s.IPAddress,
		s.CreatedAt,
		s.LastUsedAt,
		s.RevokedAt,
	).Scan(&s.ID)
	if err != nil {
		return fmt.Errorf("failed to create user session in db: %w", err)
	}
	return nil
}

// GetByID retrieves a user session by its ID.
func (r *UserSessionRepository) GetByID(ctx context.Context, id int64) (*usersession.UserSession, error) {
	query := `
		SELECT id, user_id, refresh_token_id, device_name, user_agent, ip_address, created_at, last_used_at, revoked_at
		FROM user_sessions
		WHERE id = $1
	`
	var s usersession.UserSession
	err := r.db.QueryRow(ctx, query, id).Scan(
		&s.ID,
		&s.UserID,
		&s.RefreshTokenID,
		&s.DeviceName,
		&s.UserAgent,
		&s.IPAddress,
		&s.CreatedAt,
		&s.LastUsedAt,
		&s.RevokedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.ErrSessionNotFound
		}
		return nil, fmt.Errorf("failed to get user session by id from db: %w", err)
	}
	return &s, nil
}

// GetByRefreshTokenID retrieves a user session by the associated refresh token ID.
func (r *UserSessionRepository) GetByRefreshTokenID(ctx context.Context, refreshTokenID int64) (*usersession.UserSession, error) {
	query := `
		SELECT id, user_id, refresh_token_id, device_name, user_agent, ip_address, created_at, last_used_at, revoked_at
		FROM user_sessions
		WHERE refresh_token_id = $1
	`
	var s usersession.UserSession
	err := r.db.QueryRow(ctx, query, refreshTokenID).Scan(
		&s.ID,
		&s.UserID,
		&s.RefreshTokenID,
		&s.DeviceName,
		&s.UserAgent,
		&s.IPAddress,
		&s.CreatedAt,
		&s.LastUsedAt,
		&s.RevokedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.ErrSessionNotFound
		}
		return nil, fmt.Errorf("failed to get user session by refresh token id from db: %w", err)
	}
	return &s, nil
}

// ListActiveByUserID lists active sessions for a given user.
// Active sessions are those that are not revoked and have an active, non-expired refresh token.
func (r *UserSessionRepository) ListActiveByUserID(ctx context.Context, userID int64) ([]*usersession.UserSession, error) {
	query := `
		SELECT s.id, s.user_id, s.refresh_token_id, s.device_name, s.user_agent, s.ip_address, s.created_at, s.last_used_at, s.revoked_at
		FROM user_sessions s
		JOIN refresh_tokens t ON s.refresh_token_id = t.id
		WHERE s.user_id = $1
		  AND s.revoked_at IS NULL
		  AND t.status = 'active'
		  AND t.expires_at > NOW()
		ORDER BY s.last_used_at DESC
	`
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query active sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*usersession.UserSession
	for rows.Next() {
		var s usersession.UserSession
		err := rows.Scan(
			&s.ID,
			&s.UserID,
			&s.RefreshTokenID,
			&s.DeviceName,
			&s.UserAgent,
			&s.IPAddress,
			&s.CreatedAt,
			&s.LastUsedAt,
			&s.RevokedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user session: %w", err)
		}
		sessions = append(sessions, &s)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error reading sessions rows: %w", err)
	}

	return sessions, nil
}

// Update updates a user session's mutable fields.
func (r *UserSessionRepository) Update(ctx context.Context, s *usersession.UserSession) error {
	query := `
		UPDATE user_sessions
		SET refresh_token_id = $1, last_used_at = $2, user_agent = $3, ip_address = $4, revoked_at = $5
		WHERE id = $6
	`
	res, err := r.db.Exec(ctx, query,
		s.RefreshTokenID,
		s.LastUsedAt,
		s.UserAgent,
		s.IPAddress,
		s.RevokedAt,
		s.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update user session: %w", err)
	}
	if res.RowsAffected() == 0 {
		return errs.ErrSessionNotFound
	}
	return nil
}

// Revoke marks a specific session as revoked and also revokes the associated refresh token.
func (r *UserSessionRepository) Revoke(ctx context.Context, id int64) error {
	// We use a single query with CTEs to update both tables atomically
	query := `
		WITH revoked_session AS (
			UPDATE user_sessions
			SET revoked_at = $1
			WHERE id = $2 AND revoked_at IS NULL
			RETURNING refresh_token_id
		)
		UPDATE refresh_tokens
		SET status = 'revoked'
		WHERE id = (SELECT refresh_token_id FROM revoked_session)
	`
	res, err := r.db.Exec(ctx, query, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to revoke user session and refresh token: %w", err)
	}
	if res.RowsAffected() == 0 {
		// Verify if it exists to return accurate error
		var exists bool
		err = r.db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM user_sessions WHERE id = $1)", id).Scan(&exists)
		if err == nil && !exists {
			return errs.ErrSessionNotFound
		}
	}
	return nil
}

// RevokeAllByUserID marks all user sessions as revoked and revokes all refresh tokens for that user.
func (r *UserSessionRepository) RevokeAllByUserID(ctx context.Context, userID int64) error {
	// Update all active sessions of the user to be revoked
	querySessions := `
		UPDATE user_sessions
		SET revoked_at = $1
		WHERE user_id = $2 AND revoked_at IS NULL
	`
	_, err := r.db.Exec(ctx, querySessions, time.Now(), userID)
	if err != nil {
		return fmt.Errorf("failed to revoke sessions for user: %w", err)
	}

	// Invalidate all refresh tokens for the user
	queryTokens := `
		UPDATE refresh_tokens
		SET status = 'revoked'
		WHERE user_id = $1
	`
	_, err = r.db.Exec(ctx, queryTokens, userID)
	if err != nil {
		return fmt.Errorf("failed to revoke refresh tokens for user: %w", err)
	}

	return nil
}

// Compile-time check to ensure UserSessionRepository implements usersession.UserSessionRepository
var _ usersession.UserSessionRepository = (*UserSessionRepository)(nil)
