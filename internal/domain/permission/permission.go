package permission

import (
	"context"
	"time"
)

// Permission represents a single named capability (e.g. "admin.access").
type Permission struct {
	ID          int64
	Name        string
	Description string
	CreatedAt   time.Time
}

// PermissionRepository defines the persistence contract for permissions.
// roleID is a plain int64 scalar; importing domain/role is not required,
// which keeps the dependency graph acyclic.
// Implementations live in internal/repository/postgres.
type PermissionRepository interface {
	GetByRoleID(ctx context.Context, roleID int64) ([]*Permission, error)
}

// PermissionService defines the application-level contract for permission queries.
type PermissionService interface {
	GetPermissionsByRole(ctx context.Context, roleID int64) ([]*Permission, error)
}
