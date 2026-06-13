package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/unowned-22/api/internal/domain/event"
	domainmailer "github.com/unowned-22/api/internal/domain/mailer"
	"github.com/unowned-22/api/internal/domain/passwordreset"
	"github.com/unowned-22/api/internal/domain/token"
	"github.com/unowned-22/api/internal/domain/user"
	"github.com/unowned-22/api/internal/domain/usersession"
	"github.com/unowned-22/api/internal/errs"
	"github.com/unowned-22/api/internal/infrastructure/mailer"
	"github.com/unowned-22/api/internal/logger"
	"github.com/unowned-22/api/internal/validator"
	"golang.org/x/crypto/bcrypt"
)

type PasswordResetService interface {
	RequestReset(ctx context.Context, email string) error
	ResetPassword(ctx context.Context, token, newPassword string) error
}

type passwordResetService struct {
	userRepo          user.UserRepository
	passwordResetRepo passwordreset.Repository
	refreshTokenRepo  token.RefreshTokenRepository
	userSessionRepo   usersession.UserSessionRepository
	mailer            domainmailer.Mailer
	publisher         event.Publisher
	appURL            string
	appName           string
}

func NewPasswordResetService(
	userRepo user.UserRepository,
	passwordResetRepo passwordreset.Repository,
	refreshTokenRepo token.RefreshTokenRepository,
	userSessionRepo usersession.UserSessionRepository,
	mailer domainmailer.Mailer,
	publisher event.Publisher,
	appURL string,
	appName string,
) PasswordResetService {
	return &passwordResetService{
		userRepo:          userRepo,
		passwordResetRepo: passwordResetRepo,
		refreshTokenRepo:  refreshTokenRepo,
		userSessionRepo:   userSessionRepo,
		mailer:            mailer,
		publisher:         publisher,
		appURL:            appURL,
		appName:           appName,
	}
}

func (s *passwordResetService) RequestReset(ctx context.Context, email string) error {
	if email == "" {
		return nil
	}

	u, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, errs.ErrUserNotFound) {
			return nil
		}
		return err
	}

	if err := s.passwordResetRepo.DeleteByUserID(ctx, u.ID); err != nil {
		return err
	}

	token, err := generateResetToken()
	if err != nil {
		return err
	}

	resetToken := &passwordreset.Token{
		UserID:    u.ID,
		Token:     token,
		ExpiresAt: time.Now().Add(1 * time.Hour),
		CreatedAt: time.Now(),
	}
	if err := s.passwordResetRepo.Create(ctx, resetToken); err != nil {
		return err
	}

	if err := s.sendPasswordResetEmail(ctx, u.Email, token); err != nil {
		if logger.Log != nil {
			logger.Log.WithError(err).WithFields(map[string]interface{}{
				"email":   u.Email,
				"user_id": u.ID,
			}).Warn("failed to send password reset email")
		}
	}

	// publish audit.password_reset_requested asynchronously
	go func() {
		payload, _ := json.Marshal(map[string]interface{}{"user_id": u.ID, "email": u.Email})
		if err := s.publisher.Publish(context.Background(), event.Event{Name: event.PasswordResetRequestedAudit, Payload: payload}); err != nil {
			logger.Log.WithError(err).WithFields(map[string]interface{}{"user_id": u.ID}).Warn("failed to publish audit.password_reset_requested")
		}
	}()

	return nil
}

func (s *passwordResetService) ResetPassword(ctx context.Context, token, newPassword string) error {
	if token == "" {
		return errs.ErrPasswordResetTokenInvalid
	}

	if len(newPassword) < 8 {
		return validator.Validate(struct {
			NewPassword string `validate:"required,min=8"`
		}{NewPassword: newPassword})
	}

	resetToken, err := s.passwordResetRepo.GetByToken(ctx, token)
	if err != nil {
		return err
	}

	if resetToken.ExpiresAt.Before(time.Now()) {
		return errs.ErrPasswordResetTokenInvalid
	}

	if resetToken.UsedAt != nil {
		return errs.ErrPasswordResetTokenUsed
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash new password: %w", err)
	}

	if err := s.userRepo.UpdatePassword(ctx, resetToken.UserID, string(hashedPassword)); err != nil {
		return err
	}

	if err := s.passwordResetRepo.MarkUsed(ctx, token); err != nil {
		return err
	}

	if err := s.userSessionRepo.RevokeAllByUserID(ctx, resetToken.UserID); err != nil {
		return err
	}

	// publish password_reset_completed asynchronously
	go func() {
		payload, _ := json.Marshal(map[string]interface{}{"user_id": resetToken.UserID})
		if err := s.publisher.Publish(context.Background(), event.Event{Name: event.PasswordResetCompleted, Payload: payload}); err != nil {
			logger.Log.WithError(err).WithFields(map[string]interface{}{"user_id": resetToken.UserID}).Warn("failed to publish audit.password_reset_completed")
		}
	}()

	return nil
}

func generateResetToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (s *passwordResetService) sendPasswordResetEmail(ctx context.Context, email, token string) error {
	resetURL := strings.TrimRight(s.appURL, "/") + "/reset-password?token=" + token

	htmlContent, textContent, err := mailer.RenderTemplate("reset_password", map[string]interface{}{
		"AppName":  s.appName,
		"ResetURL": resetURL,
	})
	if err != nil {
		return fmt.Errorf("failed to render password reset email template: %w", err)
	}

	msg := domainmailer.Message{
		To:      []string{email},
		Subject: "Reset Your Password",
		HTML:    htmlContent,
		Text:    textContent,
	}

	if err := s.mailer.Send(ctx, msg); err != nil {
		return fmt.Errorf("failed to send password reset email: %w", err)
	}

	return nil
}
