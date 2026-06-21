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

// Create inserts a new session and sets its ID.
func (r *UserSessionRepository) Create(ctx context.Context, s *usersession.UserSession) error {
	query := `
		INSERT INTO user_sessions (user_id, device_id, status, created_at, last_activity_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`
	err := r.db.QueryRow(ctx, query,
		s.UserID, s.DeviceID, s.Status, s.CreatedAt, s.LastActivityAt, s.ExpiresAt,
	).Scan(&s.ID)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	return nil
}

// GetByID retrieves a session by its primary key.
func (r *UserSessionRepository) GetByID(ctx context.Context, id int64) (*usersession.UserSession, error) {
	query := `
		SELECT id, user_id, device_id, status, created_at, last_activity_at, expires_at
		FROM user_sessions
		WHERE id = $1
	`
	var s usersession.UserSession
	err := r.db.QueryRow(ctx, query, id).
		Scan(&s.ID, &s.UserID, &s.DeviceID, &s.Status, &s.CreatedAt, &s.LastActivityAt, &s.ExpiresAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.ErrSessionNotFound
		}
		return nil, fmt.Errorf("failed to get session by id: %w", err)
	}
	return &s, nil
}

// ListActiveByUserID returns active, non-expired sessions for a user joined with device info,
// ordered by last_activity_at DESC.
func (r *UserSessionRepository) ListActiveByUserID(ctx context.Context, userID int64) ([]*usersession.SessionView, error) {
	query := `
		SELECT
			s.id, s.user_id, s.device_id, s.status, s.created_at, s.last_activity_at, s.expires_at,
			COALESCE(d.device_name, '') AS device_name,
			COALESCE(d.browser, '')     AS browser,
			COALESCE(d.os, '')          AS os
		FROM user_sessions s
		LEFT JOIN user_devices d ON d.id = s.device_id
		WHERE s.user_id = $1
		  AND s.status = 'active'
		  AND s.expires_at > NOW()
		ORDER BY s.last_activity_at DESC
	`
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list active sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*usersession.SessionView
	for rows.Next() {
		var sv usersession.SessionView
		err := rows.Scan(
			&sv.ID, &sv.UserID, &sv.DeviceID, &sv.Status,
			&sv.CreatedAt, &sv.LastActivityAt, &sv.ExpiresAt,
			&sv.DeviceName, &sv.Browser, &sv.OS,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan session row: %w", err)
		}
		sessions = append(sessions, &sv)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate session rows: %w", err)
	}
	return sessions, nil
}

// Terminate marks a session as revoked.
func (r *UserSessionRepository) Terminate(ctx context.Context, id int64) error {
	query := `UPDATE user_sessions SET status = 'revoked' WHERE id = $1`
	_, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to terminate session: %w", err)
	}
	return nil
}

// TerminateAll marks all non-revoked sessions for a user as revoked.
func (r *UserSessionRepository) TerminateAll(ctx context.Context, userID int64) error {
	query := `UPDATE user_sessions SET status = 'revoked' WHERE user_id = $1 AND status != 'revoked'`
	_, err := r.db.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to terminate all sessions for user: %w", err)
	}
	return nil
}

// UpdateLastActivity sets last_activity_at for a session.
func (r *UserSessionRepository) UpdateLastActivity(ctx context.Context, id int64, t time.Time) error {
	query := `UPDATE user_sessions SET last_activity_at = $1 WHERE id = $2`
	_, err := r.db.Exec(ctx, query, t, id)
	if err != nil {
		return fmt.Errorf("failed to update session last_activity_at: %w", err)
	}
	return nil
}

// Compile-time check that UserSessionRepository satisfies the domain contract.
var _ usersession.UserSessionRepository = (*UserSessionRepository)(nil)
