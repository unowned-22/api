package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/unowned-22/api/internal/domain/user"
	"github.com/unowned-22/api/internal/errs"
)

// UserRepository is the PostgreSQL implementation of user.UserRepository.
type UserRepository struct {
	db *pgxpool.Pool
}

// NewUserRepository creates a new PostgreSQL implementation of UserRepository.
func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{db: db}
}

// Create inserts a new user record including its role_id and profile fields.
func (r *UserRepository) Create(ctx context.Context, u *user.User) error {
	query := `
		INSERT INTO users (email, password, role_id, full_name, username, phone, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`
	err := r.db.QueryRow(ctx, query,
		u.Email, u.Password, u.RoleID, u.FullName, u.Username, u.Phone, u.CreatedAt,
	).Scan(&u.ID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			if pgErr.ConstraintName == "users_username_key" {
				return errs.ErrUsernameAlreadyExists
			}
			return errs.ErrUserAlreadyExists
		}
		return fmt.Errorf("failed to create user in db: %w", err)
	}
	return nil
}

// GetByEmail retrieves a user (with role name) by email address.
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*user.User, error) {
	query := `
		SELECT u.id, u.email, u.password, u.role_id, r.name, u.token_version,
		       u.full_name, u.username, COALESCE(u.phone, ''),
	       u.created_at,
	       COALESCE(u.avatar_url, ''), COALESCE(u.cover_url, ''),
		       u.email_verified_at, u.verification_token, u.verification_token_expires_at, u.deactivated_at
		FROM users u
		JOIN roles r ON r.id = u.role_id
		WHERE u.email = $1
	`
	var u user.User
	err := r.db.QueryRow(ctx, query, email).
		Scan(&u.ID, &u.Email, &u.Password, &u.RoleID, &u.RoleName, &u.TokenVersion,
			&u.FullName, &u.Username, &u.Phone,
			&u.CreatedAt, &u.AvatarURL, &u.CoverURL,
			&u.EmailVerifiedAt, &u.VerificationToken, &u.VerificationTokenExpiresAt, &u.DeactivatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user by email from db: %w", err)
	}
	return &u, nil
}

// GetByID retrieves a user (with role name) by primary key.
func (r *UserRepository) GetByID(ctx context.Context, id int64) (*user.User, error) {
	query := `
		SELECT u.id, u.email, u.password, u.role_id, r.name, u.token_version,
		       u.full_name, u.username, COALESCE(u.phone, ''),
	       u.created_at,
	       COALESCE(u.avatar_url, ''), COALESCE(u.cover_url, ''),
		       u.email_verified_at, u.verification_token, u.verification_token_expires_at, u.deactivated_at
		FROM users u
		JOIN roles r ON r.id = u.role_id
		WHERE u.id = $1
	`
	var u user.User
	err := r.db.QueryRow(ctx, query, id).
		Scan(&u.ID, &u.Email, &u.Password, &u.RoleID, &u.RoleName, &u.TokenVersion,
			&u.FullName, &u.Username, &u.Phone,
			&u.CreatedAt, &u.AvatarURL, &u.CoverURL,
			&u.EmailVerifiedAt, &u.VerificationToken, &u.VerificationTokenExpiresAt, &u.DeactivatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user by id from db: %w", err)
	}
	return &u, nil
}

// SetVerificationToken updates the user's verification token and expiry.
func (r *UserRepository) SetVerificationToken(ctx context.Context, userID int64, token string, expiresAt time.Time) error {
	query := `
		UPDATE users
		SET verification_token = $1,
		    verification_token_expires_at = $2
		WHERE id = $3
	`
	cmd, err := r.db.Exec(ctx, query, token, expiresAt, userID)
	if err != nil {
		return fmt.Errorf("failed to set verification token: %w", err)
	}
	if cmd.RowsAffected() != 1 {
		return fmt.Errorf("no user found to set verification token")
	}
	return nil
}

// UpdatePassword updates the user's password hash.
func (r *UserRepository) UpdatePassword(ctx context.Context, userID int64, hashedPassword string) error {
	query := `
		UPDATE users
		SET password = $1
		WHERE id = $2
	`
	cmd, err := r.db.Exec(ctx, query, hashedPassword, userID)
	if err != nil {
		return fmt.Errorf("failed to update user password in db: %w", err)
	}
	if cmd.RowsAffected() != 1 {
		return fmt.Errorf("no user found to update password")
	}
	return nil
}

// IncrementTokenVersion increases the user's token_version to invalidate existing JWTs.
func (r *UserRepository) IncrementTokenVersion(ctx context.Context, userID int64) error {
	query := `
		UPDATE users
		SET token_version = token_version + 1
		WHERE id = $1
	`
	cmd, err := r.db.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to increment token_version: %w", err)
	}
	if cmd.RowsAffected() != 1 {
		return fmt.Errorf("no user found to increment token_version")
	}
	return nil
}

// UpdateProfile updates user's profile fields: full_name, username, phone.
func (r *UserRepository) UpdateProfile(ctx context.Context, userID int64, fullName, username, phone string) error {
	query := `
		UPDATE users
		SET full_name = $1,
			username = $2,
			phone = $3,
			updated_at = NOW()
		WHERE id = $4
	`

	cmd, err := r.db.Exec(ctx, query, fullName, username, phone, userID)
	if err != nil {
		if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == "23505" {
			if pgErr.ConstraintName == "users_username_key" {
				return errs.ErrUsernameAlreadyExists
			}
		}
		return fmt.Errorf("failed to update user profile: %w", err)
	}
	if cmd.RowsAffected() != 1 {
		return fmt.Errorf("no user found to update profile")
	}
	return nil
}

// GetByVerificationToken retrieves a user by verification token.
func (r *UserRepository) GetByVerificationToken(ctx context.Context, token string) (*user.User, error) {
	query := `
		SELECT u.id, u.email, u.password, u.role_id, r.name, u.token_version,
		       u.full_name, u.username, COALESCE(u.phone, ''),
		       u.created_at,
		       u.email_verified_at, u.verification_token, u.verification_token_expires_at
		FROM users u
		JOIN roles r ON r.id = u.role_id
		WHERE u.verification_token = $1
	`
	var u user.User
	err := r.db.QueryRow(ctx, query, token).
		Scan(&u.ID, &u.Email, &u.Password, &u.RoleID, &u.RoleName, &u.TokenVersion,
			&u.FullName, &u.Username, &u.Phone,
			&u.CreatedAt,
			&u.EmailVerifiedAt, &u.VerificationToken, &u.VerificationTokenExpiresAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.ErrVerificationTokenInvalid
		}
		return nil, fmt.Errorf("failed to get user by verification token from db: %w", err)
	}
	return &u, nil
}

// MarkEmailVerified sets the email verified timestamp and clears the verification token.
func (r *UserRepository) MarkEmailVerified(ctx context.Context, userID int64) error {
	query := `
		UPDATE users
		SET email_verified_at = $1,
		    verification_token = NULL,
		    verification_token_expires_at = NULL
		WHERE id = $2
	`
	cmd, err := r.db.Exec(ctx, query, time.Now(), userID)
	if err != nil {
		return fmt.Errorf("failed to mark email verified: %w", err)
	}
	if cmd.RowsAffected() != 1 {
		return fmt.Errorf("no user found to mark email verified")
	}
	return nil
}

// SetDeactivatedAt sets or clears the user's deactivated timestamp.
func (r *UserRepository) SetDeactivatedAt(ctx context.Context, userID int64, t *time.Time) error {
	query := `
		UPDATE users
		SET deactivated_at = $1
		WHERE id = $2
	`
	cmd, err := r.db.Exec(ctx, query, t, userID)
	if err != nil {
		return fmt.Errorf("failed to set deactivated_at: %w", err)
	}
	if cmd.RowsAffected() != 1 {
		return fmt.Errorf("no user found to set deactivated_at")
	}
	return nil
}

// List returns a page of users ordered by created_at desc.
func (r *UserRepository) List(ctx context.Context, offset int, limit int) ([]*user.User, error) {
	query := `
		SELECT u.id, u.email, u.password, u.role_id, r.name, u.token_version,
			   u.full_name, u.username, COALESCE(u.phone, ''),
			   u.created_at,
			   COALESCE(u.avatar_url, ''), COALESCE(u.cover_url, ''),
			   u.email_verified_at, u.verification_token, u.verification_token_expires_at, u.deactivated_at
		FROM users u
		JOIN roles r ON r.id = u.role_id
		ORDER BY u.created_at DESC
		LIMIT $1 OFFSET $2
	`
	rows, err := r.db.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()

	var out []*user.User
	for rows.Next() {
		var u user.User
		if err := rows.Scan(&u.ID, &u.Email, &u.Password, &u.RoleID, &u.RoleName, &u.TokenVersion,
			&u.FullName, &u.Username, &u.Phone,
			&u.CreatedAt, &u.AvatarURL, &u.CoverURL,
			&u.EmailVerifiedAt, &u.VerificationToken, &u.VerificationTokenExpiresAt, &u.DeactivatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan user row: %w", err)
		}
		out = append(out, &u)
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("row iteration error: %w", rows.Err())
	}
	return out, nil
}

// Count returns total number of users.
func (r *UserRepository) Count(ctx context.Context) (int64, error) {
	query := `SELECT COUNT(*) FROM users`
	var cnt int64
	if err := r.db.QueryRow(ctx, query).Scan(&cnt); err != nil {
		return 0, fmt.Errorf("failed to count users: %w", err)
	}
	return cnt, nil
}

// Compile-time check that UserRepository satisfies the domain contract.
var _ user.UserRepository = (*UserRepository)(nil)

// UpdateAvatar sets the avatar URL for a user.
func (r *UserRepository) UpdateAvatar(ctx context.Context, userID int64, avatarURL string) error {
	query := `UPDATE users SET avatar_url = $1, updated_at = NOW() WHERE id = $2`
	cmd, err := r.db.Exec(ctx, query, avatarURL, userID)
	if err != nil {
		return fmt.Errorf("failed to update avatar_url: %w", err)
	}
	if cmd.RowsAffected() != 1 {
		return fmt.Errorf("no user found to update avatar")
	}
	return nil
}

// UpdateCover sets the cover URL for a user.
func (r *UserRepository) UpdateCover(ctx context.Context, userID int64, coverURL string) error {
	query := `UPDATE users SET cover_url = $1, updated_at = NOW() WHERE id = $2`
	cmd, err := r.db.Exec(ctx, query, coverURL, userID)
	if err != nil {
		return fmt.Errorf("failed to update cover_url: %w", err)
	}
	if cmd.RowsAffected() != 1 {
		return fmt.Errorf("no user found to update cover")
	}
	return nil
}
