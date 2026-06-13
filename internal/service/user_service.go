package service

import (
	"context"

	"github.com/unowned-22/api/internal/validator"

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

// UpdateProfile validates and updates the user's profile fields.
func (s *UserService) UpdateProfile(ctx context.Context, userID int64, fullName, username, phone string) error {
	if err := validator.Validate(struct {
		FullName string `validate:"required,min=2,max=100"`
		Username string `validate:"required,min=3,max=30,username"`
		Phone    string `validate:"omitempty,phone"`
	}{FullName: fullName, Username: username, Phone: phone}); err != nil {
		return err
	}

	return s.repo.UpdateProfile(ctx, userID, fullName, username, phone)
}

// Compile-time check that UserService satisfies the domain contract.
var _ user.UserService = (*UserService)(nil)
