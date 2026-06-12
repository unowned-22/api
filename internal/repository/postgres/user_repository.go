package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	domain "github.com/unowned-22/api/internal/domain/user"
	"github.com/unowned-22/api/internal/errs"
)

type UserRepository struct {
	db *pgxpool.Pool
}

// NewUserRepository creates a new PostgreSQL implementation of UserRepository.
func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{db: db}
}

// Create inserts a new user record including its role_id.
func (r *UserRepository) Create(ctx context.Context, u *domain.User) error {
	query := `
		INSERT INTO users (email, password, role_id, created_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`
	err := r.db.QueryRow(ctx, query, u.Email, u.Password, u.RoleID, u.CreatedAt).Scan(&u.ID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return errs.ErrUserAlreadyExists
		}
		return fmt.Errorf("failed to create user in db: %w", err)
	}
	return nil
}

// GetByEmail retrieves a user (with role name) by email address.
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	query := `
		SELECT u.id, u.email, u.password, u.role_id, r.name, u.created_at
		FROM users u
		JOIN roles r ON r.id = u.role_id
		WHERE u.email = $1
	`
	var u domain.User
	err := r.db.QueryRow(ctx, query, email).
		Scan(&u.ID, &u.Email, &u.Password, &u.RoleID, &u.RoleName, &u.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user by email from db: %w", err)
	}
	return &u, nil
}

// GetByID retrieves a user (with role name) by primary key.
func (r *UserRepository) GetByID(ctx context.Context, id int64) (*domain.User, error) {
	query := `
		SELECT u.id, u.email, u.password, u.role_id, r.name, u.created_at
		FROM users u
		JOIN roles r ON r.id = u.role_id
		WHERE u.id = $1
	`
	var u domain.User
	err := r.db.QueryRow(ctx, query, id).
		Scan(&u.ID, &u.Email, &u.Password, &u.RoleID, &u.RoleName, &u.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user by id from db: %w", err)
	}
	return &u, nil
}

// Ensure UserRepository implements domain.UserRepository.
var _ domain.UserRepository = (*UserRepository)(nil)
