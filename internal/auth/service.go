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
	"github.com/unowned-22/api/internal/domain/userdevice"
	"github.com/unowned-22/api/internal/domain/usersession"
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
	Email      string
	Password   string
	DeviceName string
	UserAgent  string
	IPAddress  string
}

// AuthService defines the application-level contract for authentication.
type AuthService interface {
	Register(ctx context.Context, req RegisterRequest) error
	VerifyEmail(ctx context.Context, token string) error
	ResendVerification(ctx context.Context, email string) error
	Login(ctx context.Context, req LoginRequest) (string, string, error)
	Refresh(ctx context.Context, refreshToken string, userAgent string, ipAddress string) (string, string, error)
	Logout(ctx context.Context, refreshToken string) error
	LogoutAll(ctx context.Context, userID int64) error
	DeactivateUser(ctx context.Context, userID int64) error
	ReactivateUser(ctx context.Context, userID int64) error
	ListSessions(ctx context.Context, userID int64) ([]*usersession.UserSession, error)
	RevokeSession(ctx context.Context, sessionID int64, userID int64, userRole string) error
	ChangePassword(ctx context.Context, userID int64, currentPassword, newPassword string) error
}

type authService struct {
	userRepo         user.UserRepository
	refreshTokenRepo token.RefreshTokenRepository
	userSessionRepo  usersession.UserSessionRepository
	userDeviceRepo   userdevice.Repository
	roleRepo         role.RoleRepository
	tokenManager     token.ManagerExtended
	mailer           domainmailer.Mailer
	publisher        event.Publisher
	refreshTokenTTL  time.Duration
	appURL           string
	appName          string
}

// NewAuthService wires up an AuthService with its required dependencies.
func NewAuthService(
	userRepo user.UserRepository,
	refreshTokenRepo token.RefreshTokenRepository,
	userSessionRepo usersession.UserSessionRepository,
	userDeviceRepo userdevice.Repository,
	roleRepo role.RoleRepository,
	tokenManager token.ManagerExtended,
	mailer domainmailer.Mailer,
	publisher event.Publisher,
	refreshTokenTTL time.Duration,
	appURL string,
	appName string,
) AuthService {
	return &authService{
		userRepo:         userRepo,
		refreshTokenRepo: refreshTokenRepo,
		userSessionRepo:  userSessionRepo,
		userDeviceRepo:   userDeviceRepo,
		roleRepo:         roleRepo,
		tokenManager:     tokenManager,
		mailer:           mailer,
		publisher:        publisher,
		refreshTokenTTL:  refreshTokenTTL,
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

		// publish verification_sent audit event asynchronously
		go func() {
			payload, _ := json.Marshal(map[string]interface{}{"user_id": u.ID, "email": u.Email})
			if err := s.publisher.Publish(context.Background(), event.Event{Name: event.VerificationSent, Payload: payload}); err != nil {
				logger.Log.WithError(err).WithFields(map[string]interface{}{"user_id": u.ID}).Warn("failed to publish audit.verification_sent")
			}
		}()
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

	if err := s.userRepo.MarkEmailVerified(ctx, u.ID); err != nil {
		return err
	}

	// publish email_verified audit event asynchronously
	go func() {
		payload, _ := json.Marshal(map[string]interface{}{"user_id": u.ID})
		if err := s.publisher.Publish(context.Background(), event.Event{Name: event.EmailVerified, Payload: payload}); err != nil {
			logger.Log.WithError(err).WithFields(map[string]interface{}{"user_id": u.ID}).Warn("failed to publish audit.email_verified")
		}
	}()

	return nil
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

	// publish verification_sent audit event asynchronously
	go func() {
		payload, _ := json.Marshal(map[string]interface{}{"user_id": u.ID, "email": u.Email})
		if err := s.publisher.Publish(context.Background(), event.Event{Name: event.VerificationSent, Payload: payload}); err != nil {
			logger.Log.WithError(err).WithFields(map[string]interface{}{"user_id": u.ID}).Warn("failed to publish audit.verification_sent")
		}
	}()

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

	// Deny login for deactivated users
	if u.DeactivatedAt != nil {
		return "", "", errs.ErrUserDeactivated
	}

	if err = bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(req.Password)); err != nil {
		// publish login_failed asynchronously
		go func() {
			payload, _ := json.Marshal(map[string]interface{}{
				"email":      req.Email,
				"ip_address": req.IPAddress,
				"user_agent": req.UserAgent,
			})
			if err := s.publisher.Publish(context.Background(), event.Event{Name: event.LoginFailed, Payload: payload}); err != nil {
				logger.Log.WithError(err).Warn("failed to publish audit.login_failed")
			}
		}()
		return "", "", errs.ErrInvalidCredentials
	}

	if u.EmailVerifiedAt == nil {
		return "", "", errs.ErrEmailNotVerified
	}

	// Embed role in the access token.
	accessToken, err := s.tokenManager.GenerateWithRole(u.ID, u.RoleName, u.TokenVersion)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate access token: %w", err)
	}

	refreshTokenStr, err := generateRefreshToken()
	if err != nil {
		return "", "", fmt.Errorf("failed to generate refresh token: %w", err)
	}

	rt := &token.RefreshToken{
		UserID:    u.ID,
		TokenHash: HashRefreshToken(refreshTokenStr),
		ExpiresAt: time.Now().Add(s.refreshTokenTTL),
		Status:    token.RefreshTokenStatusActive,
		CreatedAt: time.Now(),
	}

	if err = s.refreshTokenRepo.CreateRefreshToken(ctx, rt); err != nil {
		return "", "", fmt.Errorf("failed to save refresh token: %w", err)
	}

	deviceName := req.DeviceName
	if deviceName == "" {
		deviceName = "Unknown Device"
	}
	userAgent := req.UserAgent
	if userAgent == "" {
		userAgent = "Unknown"
	}
	ipAddress := req.IPAddress
	if ipAddress == "" {
		ipAddress = "Unknown"
	}

	// Device / notification handling: detect new device/browser/country
	isNewDevice := false
	if s.userDeviceRepo != nil {
		fingerprint := deviceName + "|" + userAgent
		browser := userAgent
		country := ""

		d, err := s.userDeviceRepo.GetByUnique(u.ID, fingerprint, browser, country)
		if err != nil {
			logger.Log.WithError(err).WithFields(map[string]interface{}{"user_id": u.ID}).Warn("failed to lookup user device")
		} else if d == nil {
			// new device -> create record and mark for notification
			newD := &userdevice.Device{
				UserID:      u.ID,
				Fingerprint: fingerprint,
				Browser:     browser,
				Platform:    "",
				Country:     country,
				City:        "",
				IP:          ipAddress,
				LastSeen:    time.Now(),
				CreatedAt:   time.Now(),
			}
			if err := s.userDeviceRepo.Create(newD); err != nil {
				logger.Log.WithError(err).WithFields(map[string]interface{}{"user_id": u.ID}).Warn("failed to create user device record")
			} else {
				isNewDevice = true
			}
		} else {
			// existing device — update last_seen

			if err := s.userDeviceRepo.UpdateLastSeen(d.ID, time.Now()); err != nil {
				logger.Log.WithError(err).WithFields(map[string]interface{}{"device_id": d.ID}).Warn("failed to update device last_seen")
			}
		}
	}

	// Publish login_success audit event asynchronously (include new_device flag)
	go func() {
		payload, _ := json.Marshal(map[string]interface{}{
			"user_id":    u.ID,
			"ip_address": ipAddress,
			"user_agent": userAgent,
			"device":     deviceName,
			"new_device": isNewDevice,
		})
		if err := s.publisher.Publish(context.Background(), event.Event{Name: event.LoginSuccess, Payload: payload}); err != nil {
			logger.Log.WithError(err).WithFields(map[string]interface{}{"user_id": u.ID}).Warn("failed to publish audit.login_success")
		}
	}()

	// If it's a new device, render notification email template and persist send request to outbox (avoid spamming)
	if isNewDevice {
		htmlContent, textContent, err := mailer.RenderTemplate("login_notification", map[string]interface{}{
			"AppName":  s.appName,
			"Time":     time.Now().Format(time.RFC3339),
			"IP":       ipAddress,
			"Country":  "",
			"City":     "",
			"Device":   deviceName,
			"Browser":  userAgent,
			"Platform": "",
		})
		if err != nil {
			logger.Log.WithError(err).WithFields(map[string]interface{}{"user_id": u.ID}).Warn("failed to render login notification template")
		} else {
			payload, _ := json.Marshal(map[string]interface{}{
				"to":      []string{u.Email},
				"subject": "New sign-in to your account",
				"html":    htmlContent,
				"text":    textContent,
			})

			if err := s.publisher.Publish(context.Background(), event.Event{Name: event.EmailSend, Payload: payload}); err != nil {
				logger.Log.WithError(err).WithFields(map[string]interface{}{"user_id": u.ID}).Warn("failed to publish email.send to outbox")
			}
		}
	}

	session := &usersession.UserSession{
		UserID:         u.ID,
		RefreshTokenID: rt.ID,
		DeviceName:     deviceName,
		UserAgent:      userAgent,
		IPAddress:      ipAddress,
		CreatedAt:      time.Now(),
		LastUsedAt:     time.Now(),
	}

	if err = s.userSessionRepo.Create(ctx, session); err != nil {
		return "", "", fmt.Errorf("failed to create user session: %w", err)
	}

	return accessToken, refreshTokenStr, nil
}

// ChangePassword updates the user's password after verifying the current password.
func (s *authService) ChangePassword(ctx context.Context, userID int64, currentPassword, newPassword string) error {
	u, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(currentPassword)); err != nil {
		return errs.ErrInvalidCredentials
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash new password: %w", err)
	}

	if err := s.userRepo.UpdatePassword(ctx, userID, string(hashed)); err != nil {
		return err
	}

	// Increment token version to invalidate existing JWTs
	if err := s.userRepo.IncrementTokenVersion(ctx, userID); err != nil {
		return fmt.Errorf("failed to increment token version: %w", err)
	}

	// Revoke all refresh tokens to force re-login on all devices
	if err := s.refreshTokenRepo.RevokeAllByUserID(ctx, userID); err != nil {
		return fmt.Errorf("failed to revoke refresh tokens: %w", err)
	}

	// Revoke all user sessions
	if err := s.userSessionRepo.RevokeAllByUserID(ctx, userID); err != nil {
		return fmt.Errorf("failed to revoke user sessions: %w", err)
	}

	// Publish audit event asynchronously
	go func() {
		payload, _ := json.Marshal(map[string]interface{}{"user_id": userID})
		if err := s.publisher.Publish(context.Background(), event.Event{Name: event.PasswordChanged, Payload: payload}); err != nil {
			logger.Log.WithError(err).WithFields(map[string]interface{}{"user_id": userID}).Warn("failed to publish audit.password_changed")
		}
	}()

	return nil
}

// Refresh validates a refresh token and issues a new access token (with role).
func (s *authService) Refresh(ctx context.Context, refreshTokenStr string, userAgent string, ipAddress string) (string, string, error) {
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
		// If the token expired, treat as invalid
		if rt.EffectiveStatus() == token.RefreshTokenStatusExpired {
			return "", "", errs.ErrInvalidRefreshToken
		}

		// If the token was revoked, this may indicate reuse of an old token (token theft)
		if rt.Status == token.RefreshTokenStatusRevoked {
			// Attempt to fetch user info for notification
			var userEmail string
			if u, err := s.userRepo.GetByID(ctx, rt.UserID); err == nil {
				userEmail = u.Email
			}

			// Revoke all sessions and refresh tokens for the user proactively
			if err := s.userSessionRepo.RevokeAllByUserID(ctx, rt.UserID); err != nil {
				logger.Log.WithError(err).WithFields(map[string]interface{}{"user_id": rt.UserID}).Warn("failed to revoke all user sessions during refresh-token-reuse handling")
			}
			if err := s.refreshTokenRepo.RevokeAllByUserID(ctx, rt.UserID); err != nil {
				logger.Log.WithError(err).WithFields(map[string]interface{}{"user_id": rt.UserID}).Warn("failed to revoke all refresh tokens during refresh-token-reuse handling")
			}

			// Publish audit event for reuse detection
			go func() {
				payload, _ := json.Marshal(map[string]interface{}{
					"user_id":    rt.UserID,
					"ip_address": ipAddress,
					"user_agent": userAgent,
					"token_hash": rt.TokenHash,
				})
				if err := s.publisher.Publish(context.Background(), event.Event{Name: event.RefreshTokenReuseDetected, Payload: payload}); err != nil {
					logger.Log.WithError(err).WithFields(map[string]interface{}{"user_id": rt.UserID}).Warn("failed to publish audit.refresh_token_reuse_detected")
				}
			}()

			// Optionally notify user by email
			if userEmail != "" {
				go func() {
					msg := domainmailer.Message{
						To:      []string{userEmail},
						Subject: "Security alert: possible account compromise",
						Text:    "We detected the reuse of a previously revoked refresh token for your account. All sessions have been revoked. If this wasn't you, please reset your password immediately.",
					}
					if err := s.mailer.Send(context.Background(), msg); err != nil {
						logger.Log.WithError(err).WithFields(map[string]interface{}{"user_id": rt.UserID}).Warn("failed to send refresh-token-reuse notification email")
					}
				}()
			}

			return "", "", errs.ErrInvalidRefreshToken
		}

		return "", "", errs.ErrInvalidRefreshToken
	}

	// Retrieve session associated with this refresh token and verify it is not revoked.
	session, err := s.userSessionRepo.GetByRefreshTokenID(ctx, rt.ID)
	if err != nil {
		if errors.Is(err, errs.ErrSessionNotFound) {
			return "", "", errs.ErrInvalidRefreshToken
		}
		return "", "", err
	}

	if session.RevokedAt != nil {
		return "", "", errs.ErrInvalidRefreshToken
	}

	// Rotate the refresh token immediately after validation.
	if err := s.refreshTokenRepo.RevokeRefreshToken(ctx, refreshTokenStr); err != nil {
		if errors.Is(err, errs.ErrRefreshTokenNotFound) {
			return "", "", errs.ErrInvalidRefreshToken
		}
		return "", "", err
	}

	// publish refresh_rotated asynchronously
	go func() {
		payload, _ := json.Marshal(map[string]interface{}{
			"user_id":    rt.UserID,
			"ip_address": ipAddress,
			"user_agent": userAgent,
		})
		if err := s.publisher.Publish(context.Background(), event.Event{Name: event.RefreshRotated, Payload: payload}); err != nil {
			logger.Log.WithError(err).WithFields(map[string]interface{}{"user_id": rt.UserID}).Warn("failed to publish audit.refresh_rotated")
		}
	}()

	// Look up user to get current role (role may have changed since last login).
	u, err := s.userRepo.GetByID(ctx, rt.UserID)
	if err != nil {
		return "", "", fmt.Errorf("failed to fetch user for refresh: %w", err)
	}

	// Deny refresh for deactivated users
	if u.DeactivatedAt != nil {
		return "", "", errs.ErrUserDeactivated
	}

	accessToken, err := s.tokenManager.GenerateWithRole(u.ID, u.RoleName, u.TokenVersion)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate access token: %w", err)
	}

	newRefreshTokenStr, err := generateRefreshToken()
	if err != nil {
		return "", "", fmt.Errorf("failed to generate refresh token: %w", err)
	}

	newRT := &token.RefreshToken{
		UserID:    u.ID,
		TokenHash: HashRefreshToken(newRefreshTokenStr),
		ExpiresAt: time.Now().Add(s.refreshTokenTTL),
		Status:    token.RefreshTokenStatusActive,
		CreatedAt: time.Now(),
	}

	if err := s.refreshTokenRepo.CreateRefreshToken(ctx, newRT); err != nil {
		return "", "", fmt.Errorf("failed to save refresh token: %w", err)
	}

	// Update the session with the new refresh token ID and update last_used_at, user_agent, and ip_address.
	if userAgent != "" {
		session.UserAgent = userAgent
	}
	if ipAddress != "" {
		session.IPAddress = ipAddress
	}
	session.RefreshTokenID = newRT.ID
	session.LastUsedAt = time.Now()

	if err := s.userSessionRepo.Update(ctx, session); err != nil {
		return "", "", fmt.Errorf("failed to update user session: %w", err)
	}

	return accessToken, newRefreshTokenStr, nil
}

// Logout revokes the given refresh token.
func (s *authService) Logout(ctx context.Context, refreshTokenStr string) error {
	if refreshTokenStr == "" {
		return errs.ErrInvalidRefreshToken
	}

	rt, err := s.refreshTokenRepo.GetByToken(ctx, refreshTokenStr)
	if err != nil {
		if errors.Is(err, errs.ErrRefreshTokenNotFound) {
			return errs.ErrInvalidRefreshToken
		}
		return err
	}

	// Retrieve session associated with this refresh token
	session, err := s.userSessionRepo.GetByRefreshTokenID(ctx, rt.ID)
	if err == nil {
		if err := s.userSessionRepo.Revoke(ctx, session.ID); err != nil {
			return err
		}
		// publish session_revoked audit event asynchronously
		go func() {
			payload, _ := json.Marshal(map[string]interface{}{
				"user_id":    session.UserID,
				"session_id": session.ID,
				"ip_address": session.IPAddress,
				"user_agent": session.UserAgent,
			})
			if err := s.publisher.Publish(context.Background(), event.Event{Name: event.SessionRevoked, Payload: payload}); err != nil {
				logger.Log.WithError(err).WithFields(map[string]interface{}{"session_id": session.ID}).Warn("failed to publish audit.session_revoked")
			}
		}()
	} else if !errors.Is(err, errs.ErrSessionNotFound) {
		return err
	} else {
		// Fallback for tokens without a session
		if err := s.refreshTokenRepo.RevokeRefreshToken(ctx, refreshTokenStr); err != nil {
			return err
		}
	}

	// publish logout event if we have token info
	if rt != nil {
		go func() {
			payload, _ := json.Marshal(map[string]interface{}{
				"user_id": rt.UserID,
			})
			if err := s.publisher.Publish(context.Background(), event.Event{Name: event.Logout, Payload: payload}); err != nil {
				logger.Log.WithError(err).WithFields(map[string]interface{}{"user_id": rt.UserID}).Warn("failed to publish audit.logout")
			}
		}()
	}
	return nil
}

// LogoutAll revokes all sessions and refresh tokens for the given user.
func (s *authService) LogoutAll(ctx context.Context, userID int64) error {
	if err := s.userSessionRepo.RevokeAllByUserID(ctx, userID); err != nil {
		return err
	}

	// publish logout_all audit event asynchronously
	go func() {
		payload, _ := json.Marshal(map[string]interface{}{"user_id": userID})
		if err := s.publisher.Publish(context.Background(), event.Event{Name: event.LogoutAll, Payload: payload}); err != nil {
			logger.Log.WithError(err).WithFields(map[string]interface{}{"user_id": userID}).Warn("failed to publish audit.logout_all")
		}
	}()

	return nil
}

// DeactivateUser marks a user as deactivated and revokes all sessions and refresh tokens.
func (s *authService) DeactivateUser(ctx context.Context, userID int64) error {
	now := time.Now()
	if err := s.userRepo.SetDeactivatedAt(ctx, userID, &now); err != nil {
		return err
	}

	if err := s.userSessionRepo.RevokeAllByUserID(ctx, userID); err != nil {
		return err
	}

	if err := s.refreshTokenRepo.RevokeAllByUserID(ctx, userID); err != nil {
		return err
	}

	// publish account_deactivated audit event asynchronously
	go func() {
		payload, _ := json.Marshal(map[string]interface{}{"user_id": userID})
		if err := s.publisher.Publish(context.Background(), event.Event{Name: event.AccountDeactivated, Payload: payload}); err != nil {
			logger.Log.WithError(err).WithFields(map[string]interface{}{"user_id": userID}).Warn("failed to publish audit.account_deactivated")
		}
	}()

	return nil
}

// ReactivateUser clears the deactivated timestamp to re-enable the account.
func (s *authService) ReactivateUser(ctx context.Context, userID int64) error {
	// Clear deactivated_at
	if err := s.userRepo.SetDeactivatedAt(ctx, userID, nil); err != nil {
		return err
	}

	// publish account_activated audit event asynchronously
	go func() {
		payload, _ := json.Marshal(map[string]interface{}{"user_id": userID})
		if err := s.publisher.Publish(context.Background(), event.Event{Name: event.AccountActivated, Payload: payload}); err != nil {
			logger.Log.WithError(err).WithFields(map[string]interface{}{"user_id": userID}).Warn("failed to publish audit.account_activated")
		}
	}()
	return nil
}

func (s *authService) ListSessions(ctx context.Context, userID int64) ([]*usersession.UserSession, error) {
	return s.userSessionRepo.ListActiveByUserID(ctx, userID)
}

func (s *authService) RevokeSession(ctx context.Context, sessionID int64, userID int64, userRole string) error {
	session, err := s.userSessionRepo.GetByID(ctx, sessionID)
	if err != nil {
		return err
	}

	// Regular users can only revoke their own sessions. Admins can revoke any session.
	if userRole != "admin" && session.UserID != userID {
		return errs.ErrForbidden
	}

	return s.userSessionRepo.Revoke(ctx, sessionID)
}

// HashRefreshToken returns the SHA-256 hash of a refresh token string.
func HashRefreshToken(refreshToken string) string {
	return token.HashRefreshToken(refreshToken)
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
