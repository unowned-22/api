package user

import (
	"context"
	"time"
)

type Permission struct {
	ID          int64
	Name        string
	Description string
	CreatedAt   time.Time
}

type PermissionRepository interface {
	GetByRoleID(ctx context.Context, roleID int64) ([]*Permission, error)
}

type PermissionService interface {
	GetPermissionsByRole(ctx context.Context, roleID int64) ([]*Permission, error)
}
