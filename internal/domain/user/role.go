package user

import (
	"context"
	"time"
)

// Role represents a user role in the system (e.g. "admin", "user").
type Role struct {
	ID        int64
	Name      string
	CreatedAt time.Time
}

// RoleRepository defines the persistence contract for roles.
// Implementations live in internal/repository/postgres.
type RoleRepository interface {
	GetByID(ctx context.Context, id int64) (*Role, error)
	GetByName(ctx context.Context, name string) (*Role, error)
	List(ctx context.Context) ([]*Role, error)
}
