package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	domain "github.com/unowned-22/api/internal/domain/user"
)

type PermissionRepository struct {
	db *pgxpool.Pool
}

func NewPermissionRepository(db *pgxpool.Pool) *PermissionRepository {
	return &PermissionRepository{db: db}
}

func (r *PermissionRepository) GetByRoleID(ctx context.Context, roleID int64) ([]*domain.Permission, error) {
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

	permissions := make([]*domain.Permission, 0)
	for rows.Next() {
		var permission domain.Permission
		if err := rows.Scan(&permission.ID, &permission.Name, &permission.Description, &permission.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan permission: %w", err)
		}
		permissions = append(permissions, &permission)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error getting permissions by role id: %w", err)
	}

	return permissions, nil
}

var _ domain.PermissionRepository = (*PermissionRepository)(nil)
