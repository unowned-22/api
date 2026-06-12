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

// Compile-time check that UserService satisfies the domain contract.
var _ user.UserService = (*UserService)(nil)
