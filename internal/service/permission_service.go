package service

import (
	"context"

	domain "github.com/unowned-22/api/internal/domain/user"
)

type PermissionService struct {
	repo domain.PermissionRepository
}

func NewPermissionService(repo domain.PermissionRepository) *PermissionService {
	return &PermissionService{repo: repo}
}

func (s *PermissionService) GetPermissionsByRole(ctx context.Context, roleID int64) ([]*domain.Permission, error) {
	return s.repo.GetByRoleID(ctx, roleID)
}

var _ domain.PermissionService = (*PermissionService)(nil)
