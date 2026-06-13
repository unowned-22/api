package service

import (
	"context"

	"github.com/unowned-22/api/internal/domain/user"
)

// UserService implements domain/user.UserService.
type UserService struct {
	repo user.UserRepository
}

// NewUserService creates a new instance of UserService.
func NewUserService(repo user.UserRepository) *UserService {
	return &UserService{repo: repo}
}

// GetProfile returns the full user record (including role) by ID.
func (s *UserService) GetProfile(ctx context.Context, userID int64) (*user.User, error) {
	return s.repo.GetByID(ctx, userID)
}

// ListUsers returns users for the requested page and limit along with total count.
func (s *UserService) ListUsers(ctx context.Context, page int, limit int) ([]*user.User, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	offset := (page - 1) * limit
	items, err := s.repo.List(ctx, offset, limit)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.repo.Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// Compile-time check that UserService satisfies the domain contract.
var _ user.UserService = (*UserService)(nil)
