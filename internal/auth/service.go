package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/unowned-22/api/internal/domain/event"
	domainmailer "github.com/unowned-22/api/internal/domain/mailer"
	"github.com/unowned-22/api/internal/domain/role"
	"github.com/unowned-22/api/internal/domain/token"
	"github.com/unowned-22/api/internal/domain/user"
	"github.com/unowned-22/api/internal/errs"
	"github.com/unowned-22/api/internal/infrastructure/mailer"
	"github.com/unowned-22/api/internal/logger"
	"golang.org/x/crypto/bcrypt"
)

type RegisterRequest struct {
	Email    string
	Password string
	FullName string
	Username string
	Phone    string
}

type LoginRequest struct {
	Email    string
	Password string
}

// AuthService defines the application-level contract for authentication.
type AuthService interface {
	Register(ctx context.Context, req RegisterRequest) error
	VerifyEmail(ctx context.Context, token string) error
	ResendVerification(ctx context.Context, email string) error
	Login(ctx context.Context, req LoginRequest) (string, string, error)
	Refresh(ctx context.Context, refreshToken string) (string, string, error)
	Logout(ctx context.Context, refreshToken string) error
}

type authService struct {
	userRepo         user.UserRepository
	refreshTokenRepo token.RefreshTokenRepository
	roleRepo         role.RoleRepository
	tokenManager     token.ManagerExtended
	mailer           domainmailer.Mailer
	publisher        event.Publisher
	appURL           string
	appName          string
}

// NewAuthService wires up an AuthService with its required dependencies.
func NewAuthService(
	userRepo user.UserRepository,
	refreshTokenRepo token.RefreshTokenRepository,
	roleRepo role.RoleRepository,
	tokenManager token.ManagerExtended,
	mailer domainmailer.Mailer,
	publisher event.Publisher,
	appURL string,
	appName string,
) AuthService {
	return &authService{
		userRepo:         userRepo,
		refreshTokenRepo: refreshTokenRepo,
		roleRepo:         roleRepo,
		tokenManager:     tokenManager,
		mailer:           mailer,
		publisher:        publisher,
		appURL:           appURL,
		appName:          appName,
	}
}

// Register hashes the user's password, resolves the default "user" role,
// persists the new user to the repository, stores a verification token,
// and sends a verification email.
func (s *authService) Register(ctx context.Context, req RegisterRequest) error {
	if req.Email == "" || req.Password == "" {
		return fmt.Errorf("email and password are required")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Resolve role ID dynamically — no hardcoded IDs allowed.
	defaultRole, err := s.roleRepo.GetByName(ctx, "user")
	if err != nil {
		return fmt.Errorf("failed to resolve default role: %w", err)
	}

	u := &user.User{
		Email:     req.Email,
		Password:  string(hashedPassword),
		RoleID:    defaultRole.ID,
		FullName:  req.FullName,
		Username:  req.Username,
		Phone:     req.Phone,
		CreatedAt: time.Now(),
	}

	if err := s.userRepo.Create(ctx, u); err != nil {
		return err
	}

	verificationToken, err := generateVerificationToken()
	if err != nil {
		return fmt.Errorf("failed to generate verification token: %w", err)
	}
	expiresAt := time.Now().Add(24 * time.Hour)
	if err := s.userRepo.SetVerificationToken(ctx, u.ID, verificationToken, expiresAt); err != nil {
		return fmt.Errorf("failed to persist verification token: %w", err)
	}

	payload, err := json.Marshal(map[string]interface{}{
		"user_id": u.ID,
		"email":   u.Email,
		"token":   verificationToken,
	})
	if err != nil {
		logger.Log.WithError(err).Warn("failed to marshal user.registered event")
	} else {
		if err := s.publisher.Publish(ctx, event.Event{
			Name:    event.UserRegistered,
			Payload: payload,
		}); err != nil {
			logger.Log.WithError(err).WithFields(map[string]interface{}{
				"email":   u.Email,
				"user_id": u.ID,
			}).Warn("failed to publish user.registered event")
		}
	}

	return nil
}

// VerifyEmail validates a verification token and marks the user's email as verified.
func (s *authService) VerifyEmail(ctx context.Context, token string) error {
	if token == "" {
		return errs.ErrVerificationTokenInvalid
	}

	u, err := s.userRepo.GetByVerificationToken(ctx, token)
	if err != nil {
		return err
	}

	if u.EmailVerifiedAt != nil {
		return errs.ErrEmailAlreadyVerified
	}

	if u.VerificationTokenExpiresAt == nil || u.VerificationTokenExpiresAt.Before(time.Now()) {
		return errs.ErrVerificationTokenInvalid
	}

	return s.userRepo.MarkEmailVerified(ctx, u.ID)
}

// ResendVerification generates a new verification token and sends it to the user's email.
func (s *authService) ResendVerification(ctx context.Context, email string) error {
	if email == "" {
		return fmt.Errorf("email is required")
	}

	u, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return err
	}

	if u.EmailVerifiedAt != nil {
		return errs.ErrEmailAlreadyVerified
	}

	verificationToken, err := generateVerificationToken()
	if err != nil {
		return fmt.Errorf("failed to generate verification token: %w", err)
	}
	expiresAt := time.Now().Add(24 * time.Hour)
	if err := s.userRepo.SetVerificationToken(ctx, u.ID, verificationToken, expiresAt); err != nil {
		return fmt.Errorf("failed to persist verification token: %w", err)
	}

	if err := s.sendVerificationEmail(ctx, u.Email, verificationToken); err != nil {
		return err
	}

	return nil
}

// Login validates credentials and returns an access token (with role claim) and a refresh token.
func (s *authService) Login(ctx context.Context, req LoginRequest) (string, string, error) {
	if req.Email == "" || req.Password == "" {
		return "", "", errs.ErrInvalidCredentials
	}

	u, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, errs.ErrUserNotFound) {
			return "", "", errs.ErrInvalidCredentials
		}
		return "", "", err
	}

	if err = bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(req.Password)); err != nil {
		return "", "", errs.ErrInvalidCredentials
	}

	if u.EmailVerifiedAt == nil {
		return "", "", errs.ErrEmailNotVerified
	}

	// Embed role in the access token.
	accessToken, err := s.tokenManager.GenerateWithRole(u.ID, u.RoleName)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate access token: %w", err)
	}

	refreshTokenStr, err := generateRefreshToken()
	if err != nil {
		return "", "", fmt.Errorf("failed to generate refresh token: %w", err)
	}

	rt := &token.RefreshToken{
		UserID:    u.ID,
		Token:     refreshTokenStr,
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
		Status:    token.RefreshTokenStatusActive,
		CreatedAt: time.Now(),
	}

	if err = s.refreshTokenRepo.CreateRefreshToken(ctx, rt); err != nil {
		return "", "", fmt.Errorf("failed to save refresh token: %w", err)
	}

	return accessToken, refreshTokenStr, nil
}

// Refresh validates a refresh token and issues a new access token (with role).
func (s *authService) Refresh(ctx context.Context, refreshTokenStr string) (string, string, error) {
	if refreshTokenStr == "" {
		return "", "", errs.ErrInvalidRefreshToken
	}

	rt, err := s.refreshTokenRepo.GetByToken(ctx, refreshTokenStr)
	if err != nil {
		if errors.Is(err, errs.ErrRefreshTokenNotFound) {
			return "", "", errs.ErrInvalidRefreshToken
		}
		return "", "", err
	}

	if rt.EffectiveStatus() != token.RefreshTokenStatusActive {
		return "", "", errs.ErrInvalidRefreshToken
	}

	// Rotate the refresh token immediately after validation.
	if err := s.refreshTokenRepo.RevokeRefreshToken(ctx, refreshTokenStr); err != nil {
		if errors.Is(err, errs.ErrRefreshTokenNotFound) {
			return "", "", errs.ErrInvalidRefreshToken
		}
		return "", "", err
	}

	// Look up user to get current role (role may have changed since last login).
	u, err := s.userRepo.GetByID(ctx, rt.UserID)
	if err != nil {
		return "", "", fmt.Errorf("failed to fetch user for refresh: %w", err)
	}

	accessToken, err := s.tokenManager.GenerateWithRole(u.ID, u.RoleName)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate access token: %w", err)
	}

	newRefreshTokenStr, err := generateRefreshToken()
	if err != nil {
		return "", "", fmt.Errorf("failed to generate refresh token: %w", err)
	}

	newRT := &token.RefreshToken{
		UserID:    u.ID,
		Token:     newRefreshTokenStr,
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
		Status:    token.RefreshTokenStatusActive,
		CreatedAt: time.Now(),
	}

	if err := s.refreshTokenRepo.CreateRefreshToken(ctx, newRT); err != nil {
		return "", "", fmt.Errorf("failed to save refresh token: %w", err)
	}

	return accessToken, newRefreshTokenStr, nil
}

// Logout revokes the given refresh token.
func (s *authService) Logout(ctx context.Context, refreshTokenStr string) error {
	if refreshTokenStr == "" {
		return errs.ErrInvalidRefreshToken
	}

	if err := s.refreshTokenRepo.RevokeRefreshToken(ctx, refreshTokenStr); err != nil {
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

// generateVerificationToken produces a cryptographically secure email verification token.
func generateVerificationToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (s *authService) sendVerificationEmail(ctx context.Context, email, token string) error {
	verificationURL := strings.TrimRight(s.appURL, "/") + "/verify-email?token=" + token

	htmlContent, textContent, err := mailer.RenderTemplate("verify_email", map[string]interface{}{
		"AppName":         s.appName,
		"VerificationURL": verificationURL,
	})
	if err != nil {
		return fmt.Errorf("failed to render verification email template: %w", err)
	}

	msg := domainmailer.Message{
		To:      []string{email},
		Subject: "Verify Your Email Address",
		HTML:    htmlContent,
		Text:    textContent,
	}

	if err := s.mailer.Send(ctx, msg); err != nil {
		return fmt.Errorf("failed to send verification email: %w", err)
	}

	return nil
}
