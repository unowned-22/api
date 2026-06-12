package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/unowned-22/api/internal/domain/role"
	"github.com/unowned-22/api/internal/errs"
)

// RoleRepository is the PostgreSQL implementation of role.RoleRepository.
type RoleRepository struct {
	db *pgxpool.Pool
}

// NewRoleRepository creates a new PostgreSQL implementation of RoleRepository.
func NewRoleRepository(db *pgxpool.Pool) *RoleRepository {
	return &RoleRepository{db: db}
}

// GetByID retrieves a role by its primary key.
func (r *RoleRepository) GetByID(ctx context.Context, id int64) (*role.Role, error) {
	query := `SELECT id, name, created_at FROM roles WHERE id = $1`
	var ro role.Role
	err := r.db.QueryRow(ctx, query, id).Scan(&ro.ID, &ro.Name, &ro.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.ErrRoleNotFound
		}
		return nil, fmt.Errorf("failed to get role by id: %w", err)
	}
	return &ro, nil
}

// GetByName retrieves a role by its unique name (e.g. "admin", "user").
func (r *RoleRepository) GetByName(ctx context.Context, name string) (*role.Role, error) {
	query := `SELECT id, name, created_at FROM roles WHERE name = $1`
	var ro role.Role
	err := r.db.QueryRow(ctx, query, name).Scan(&ro.ID, &ro.Name, &ro.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.ErrRoleNotFound
		}
		return nil, fmt.Errorf("failed to get role by name: %w", err)
	}
	return &ro, nil
}

// List returns all roles defined in the system.
func (r *RoleRepository) List(ctx context.Context) ([]*role.Role, error) {
	query := `SELECT id, name, created_at FROM roles ORDER BY id`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list roles: %w", err)
	}
	defer rows.Close()

	var roles []*role.Role
	for rows.Next() {
		var ro role.Role
		if err := rows.Scan(&ro.ID, &ro.Name, &ro.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan role: %w", err)
		}
		roles = append(roles, &ro)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error listing roles: %w", err)
	}
	return roles, nil
}

// Compile-time check that RoleRepository satisfies the domain contract.
var _ role.RoleRepository = (*RoleRepository)(nil)
