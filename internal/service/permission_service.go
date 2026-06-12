package service

import (
	"context"

	"github.com/unowned-22/api/internal/domain/permission"
)

// PermissionService implements domain/permission.PermissionService.
type PermissionService struct {
	repo permission.PermissionRepository
}

// NewPermissionService creates a new instance of PermissionService.
func NewPermissionService(repo permission.PermissionRepository) *PermissionService {
	return &PermissionService{repo: repo}
}

// GetPermissionsByRole returns all permissions assigned to the given role.
func (s *PermissionService) GetPermissionsByRole(ctx context.Context, roleID int64) ([]*permission.Permission, error) {
	return s.repo.GetByRoleID(ctx, roleID)
}

// Compile-time check that PermissionService satisfies the domain contract.
var _ permission.PermissionService = (*PermissionService)(nil)
