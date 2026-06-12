package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/unowned-22/api/internal/domain/permission"
)

// PermissionRepository is the PostgreSQL implementation of permission.PermissionRepository.
type PermissionRepository struct {
	db *pgxpool.Pool
}

// NewPermissionRepository creates a new PostgreSQL implementation of PermissionRepository.
func NewPermissionRepository(db *pgxpool.Pool) *PermissionRepository {
	return &PermissionRepository{db: db}
}

// GetByRoleID returns all permissions assigned to the given role.
func (r *PermissionRepository) GetByRoleID(ctx context.Context, roleID int64) ([]*permission.Permission, error) {
	query := `
		SELECT p.id, p.name, COALESCE(p.description, ''), p.created_at
		FROM permissions p
		JOIN role_permissions rp ON rp.permission_id = p.id
		WHERE rp.role_id = $1
		ORDER BY p.name
	`
	rows, err := r.db.Query(ctx, query, roleID)
	if err != nil {
		return nil, fmt.Errorf("failed to get permissions by role id: %w", err)
	}
	defer rows.Close()

	permissions := make([]*permission.Permission, 0)
	for rows.Next() {
		var p permission.Permission
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan permission: %w", err)
		}
		permissions = append(permissions, &p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error getting permissions by role id: %w", err)
	}

	return permissions, nil
}

// Compile-time check that PermissionRepository satisfies the domain contract.
var _ permission.PermissionRepository = (*PermissionRepository)(nil)
