package service

import (
	"context"

	domain "github.com/unowned-22/api/internal/domain/user"
)

type UserService struct {
	repo domain.UserRepository
}

// NewUserService creates a new instance of UserService.
func NewUserService(repo domain.UserRepository) *UserService {
	return &UserService{
		repo: repo,
	}
}

// GetProfile returns the full user record (including role) by ID.
func (s *UserService) GetProfile(ctx context.Context, userID int64) (*domain.User, error) {
	return s.repo.GetByID(ctx, userID)
}

// Ensure UserService satisfies the domain contract.
var _ domain.UserService = (*UserService)(nil)
