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

type RoleRepository struct {
	db *pgxpool.Pool
}

// NewRoleRepository creates a new PostgreSQL implementation of RoleRepository.
func NewRoleRepository(db *pgxpool.Pool) *RoleRepository {
	return &RoleRepository{db: db}
}

// GetByID retrieves a role by its primary key.
func (r *RoleRepository) GetByID(ctx context.Context, id int64) (*domain.Role, error) {
	query := `SELECT id, name, created_at FROM roles WHERE id = $1`
	var role domain.Role
	err := r.db.QueryRow(ctx, query, id).Scan(&role.ID, &role.Name, &role.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.ErrRoleNotFound
		}
		return nil, fmt.Errorf("failed to get role by id: %w", err)
	}
	return &role, nil
}

// GetByName retrieves a role by its unique name (e.g. "admin", "user").
func (r *RoleRepository) GetByName(ctx context.Context, name string) (*domain.Role, error) {
	query := `SELECT id, name, created_at FROM roles WHERE name = $1`
	var role domain.Role
	err := r.db.QueryRow(ctx, query, name).Scan(&role.ID, &role.Name, &role.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.ErrRoleNotFound
		}
		return nil, fmt.Errorf("failed to get role by name: %w", err)
	}
	return &role, nil
}

// List returns all roles defined in the system.
func (r *RoleRepository) List(ctx context.Context) ([]*domain.Role, error) {
	query := `SELECT id, name, created_at FROM roles ORDER BY id`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list roles: %w", err)
	}
	defer rows.Close()

	var roles []*domain.Role
	for rows.Next() {
		var role domain.Role
		if err := rows.Scan(&role.ID, &role.Name, &role.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan role: %w", err)
		}
		roles = append(roles, &role)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error listing roles: %w", err)
	}
	return roles, nil
}

// Ensure RoleRepository implements domain.RoleRepository.
var _ domain.RoleRepository = (*RoleRepository)(nil)
