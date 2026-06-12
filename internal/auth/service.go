package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	domain "github.com/unowned-22/api/internal/domain/user"
	"github.com/unowned-22/api/internal/errs"
	"golang.org/x/crypto/bcrypt"
)

type RegisterRequest struct {
	Email    string
	Password string
}

type LoginRequest struct {
	Email    string
	Password string
}

type AuthService interface {
	Register(ctx context.Context, req RegisterRequest) error
	Login(ctx context.Context, req LoginRequest) (string, string, error)
	Refresh(ctx context.Context, refreshToken string) (string, error)
	Logout(ctx context.Context, refreshToken string) error
}

type authService struct {
	repo             domain.UserRepository
	refreshTokenRepo domain.RefreshTokenRepository
	roleRepo         domain.RoleRepository
	tokenManager     domain.TokenManagerExtended
}

func NewAuthService(
	repo domain.UserRepository,
	refreshTokenRepo domain.RefreshTokenRepository,
	roleRepo domain.RoleRepository,
	tokenManager domain.TokenManagerExtended,
) AuthService {
	return &authService{
		repo:             repo,
		refreshTokenRepo: refreshTokenRepo,
		roleRepo:         roleRepo,
		tokenManager:     tokenManager,
	}
}

// Register hashes the user's password, resolves the default "user" role,
// and persists the new user to the repository.
func (s *authService) Register(ctx context.Context, req RegisterRequest) error {
	if req.Email == "" || req.Password == "" {
		return fmt.Errorf("email and password are required")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Resolve role ID dynamically — no hardcoded IDs allowed.
	role, err := s.roleRepo.GetByName(ctx, "user")
	if err != nil {
		return fmt.Errorf("failed to resolve default role: %w", err)
	}

	user := &domain.User{
		Email:     req.Email,
		Password:  string(hashedPassword),
		RoleID:    role.ID,
		CreatedAt: time.Now(),
	}

	return s.repo.Create(ctx, user)
}

// Login validates credentials and returns an access token (with role claim) and a refresh token.
func (s *authService) Login(ctx context.Context, req LoginRequest) (string, string, error) {
	if req.Email == "" || req.Password == "" {
		return "", "", errs.ErrInvalidCredentials
	}

	user, err := s.repo.GetByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, errs.ErrUserNotFound) {
			return "", "", errs.ErrInvalidCredentials
		}
		return "", "", err
	}

	if err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return "", "", errs.ErrInvalidCredentials
	}

	// Embed role in the access token.
	accessToken, err := s.tokenManager.GenerateWithRole(user.ID, user.RoleName)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate access token: %w", err)
	}

	refreshTokenStr, err := generateRefreshToken()
	if err != nil {
		return "", "", fmt.Errorf("failed to generate refresh token: %w", err)
	}

	refreshToken := &domain.RefreshToken{
		UserID:    user.ID,
		Token:     refreshTokenStr,
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
		Revoked:   false,
		CreatedAt: time.Now(),
	}

	if err = s.refreshTokenRepo.Create(ctx, refreshToken); err != nil {
		return "", "", fmt.Errorf("failed to save refresh token: %w", err)
	}

	return accessToken, refreshTokenStr, nil
}

// Refresh validates a refresh token and issues a new access token (with role).
func (s *authService) Refresh(ctx context.Context, refreshTokenStr string) (string, error) {
	if refreshTokenStr == "" {
		return "", errs.ErrInvalidRefreshToken
	}

	token, err := s.refreshTokenRepo.GetByToken(ctx, refreshTokenStr)
	if err != nil {
		if errors.Is(err, errs.ErrRefreshTokenNotFound) {
			return "", errs.ErrInvalidRefreshToken
		}
		return "", err
	}

	if token.Revoked {
		return "", errs.ErrInvalidRefreshToken
	}
	if token.ExpiresAt.Before(time.Now()) {
		return "", errs.ErrInvalidRefreshToken
	}

	// Look up user to get current role (role may have changed since last login).
	user, err := s.repo.GetByID(ctx, token.UserID)
	if err != nil {
		return "", fmt.Errorf("failed to fetch user for refresh: %w", err)
	}

	accessToken, err := s.tokenManager.GenerateWithRole(user.ID, user.RoleName)
	if err != nil {
		return "", fmt.Errorf("failed to generate access token: %w", err)
	}

	return accessToken, nil
}

// Logout revokes the given refresh token.
func (s *authService) Logout(ctx context.Context, refreshTokenStr string) error {
	if refreshTokenStr == "" {
		return errs.ErrInvalidRefreshToken
	}

	if err := s.refreshTokenRepo.Revoke(ctx, refreshTokenStr); err != nil {
		if errors.Is(err, errs.ErrRefreshTokenNotFound) {
			return errs.ErrInvalidRefreshToken
		}
		return err
	}
	return nil
}

// generateRefreshToken produces a cryptographically secure opaque token.
func generateRefreshToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
