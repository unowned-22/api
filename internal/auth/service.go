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

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/unowned-22/api/internal/database"
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
	OS         string
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
	ListSessions(ctx context.Context, userID int64) ([]*usersession.SessionView, error)
	RevokeSession(ctx context.Context, sessionID int64, userID int64, userRole string) error
	ChangePassword(ctx context.Context, userID int64, currentPassword, newPassword string) error
}

type authService struct {
	userRepo          user.UserRepository
	refreshTokenRepo  token.RefreshTokenRepository
	userSessionRepo   usersession.UserSessionRepository
	userDeviceRepo    userdevice.Repository
	roleRepo          role.RoleRepository
	tokenManager      token.ManagerExtended
	mailer            domainmailer.Mailer
	publisher         event.Publisher
	pool              *pgxpool.Pool // used only for the Register transaction
	refreshTokenTTL   time.Duration
	appURL            string
	appName           string
	tokenVersionCache user.TokenVersionCache
}

// NewAuthService wires up an AuthService with its required dependencies.
// pool is required only for the atomic registration transaction; all other
// operations go through repository interfaces.
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
	tokenVersionCache user.TokenVersionCache,
	pool *pgxpool.Pool,
) AuthService {
	return &authService{
		userRepo:          userRepo,
		refreshTokenRepo:  refreshTokenRepo,
		userSessionRepo:   userSessionRepo,
		userDeviceRepo:    userDeviceRepo,
		roleRepo:          roleRepo,
		tokenManager:      tokenManager,
		mailer:            mailer,
		publisher:         publisher,
		pool:              pool,
		refreshTokenTTL:   refreshTokenTTL,
		appURL:            appURL,
		appName:           appName,
		tokenVersionCache: tokenVersionCache,
	}
}

// Register hashes the user's password, resolves the default "user" role,
// persists the new user, stores a verification token, and publishes a
// user.registered outbox event — all atomically in a single transaction.
func (s *authService) Register(ctx context.Context, req RegisterRequest) error {
	if req.Email == "" || req.Password == "" {
		return fmt.Errorf("email and password are required")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

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

	verificationToken, err := generateVerificationToken()
	if err != nil {
		return fmt.Errorf("failed to generate verification token: %w", err)
	}
	expiresAt := time.Now().Add(24 * time.Hour)

	// txUserRepo asserts the repo supports transactional writes.
	txUserRepo, hasTx := s.userRepo.(interface {
		CreateTx(ctx context.Context, tx pgx.Tx, u *user.User) error
		SetVerificationTokenTx(ctx context.Context, tx pgx.Tx, userID int64, token string, expiresAt time.Time) error
	})

	// txPublisher asserts the publisher supports writing into the outbox within a tx.
	txPublisher, hasTxPub := s.publisher.(interface {
		PublishTx(ctx context.Context, tx pgx.Tx, e event.Event) error
	})

	if hasTx && hasTxPub && s.pool != nil {
		// Production path: atomic transaction.
		err = database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
			if err := txUserRepo.CreateTx(ctx, tx, u); err != nil {
				return err
			}
			if err := txUserRepo.SetVerificationTokenTx(ctx, tx, u.ID, verificationToken, expiresAt); err != nil {
				return err
			}
			payload, err := json.Marshal(map[string]interface{}{
				"user_id": u.ID,
				"email":   u.Email,
				"token":   verificationToken,
			})
			if err != nil {
				return fmt.Errorf("failed to marshal user.registered event: %w", err)
			}
			return txPublisher.PublishTx(ctx, tx, event.Event{
				Name:    event.UserRegistered,
				Payload: payload,
			})
		})
	} else {
		// Fallback path: sequential calls (used in tests with mock repositories).
		if err = s.userRepo.Create(ctx, u); err != nil {
			return err
		}
		if err = s.userRepo.SetVerificationToken(ctx, u.ID, verificationToken, expiresAt); err != nil {
			return fmt.Errorf("failed to persist verification token: %w", err)
		}
		payload, jsonErr := json.Marshal(map[string]interface{}{
			"user_id": u.ID,
			"email":   u.Email,
			"token":   verificationToken,
		})
		if jsonErr != nil {
			return fmt.Errorf("failed to marshal user.registered event: %w", jsonErr)
		}
		if pubErr := s.publisher.Publish(ctx, event.Event{Name: event.UserRegistered, Payload: payload}); pubErr != nil {
			logger.Log.WithError(pubErr).WithFields(map[string]interface{}{"user_id": u.ID}).Warn("failed to publish user.registered event")
		}
	}
	if err != nil {
		return err
	}

	// Best-effort audit event published after the transaction commits.
	auditPayload, _ := json.Marshal(map[string]interface{}{"user_id": u.ID, "email": u.Email})
	if err := s.publisher.Publish(ctx, event.Event{Name: event.VerificationSent, Payload: auditPayload}); err != nil {
		logger.Log.WithError(err).WithFields(map[string]interface{}{"user_id": u.ID}).Warn("failed to publish audit.verification_sent")
	}

	return nil
}

// VerifyEmail validates a verification token and marks the user's email as verified.
func (s *authService) VerifyEmail(ctx context.Context, verificationToken string) error {
	if verificationToken == "" {
		return errs.ErrVerificationTokenInvalid
	}

	u, err := s.userRepo.GetByVerificationToken(ctx, verificationToken)
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

	// publish email_verified audit event
	payload, _ := json.Marshal(map[string]interface{}{"user_id": u.ID})
	if err := s.publisher.Publish(ctx, event.Event{Name: event.EmailVerified, Payload: payload}); err != nil {
		logger.Log.WithError(err).WithFields(map[string]interface{}{"user_id": u.ID}).Warn("failed to publish audit.email_verified")
	}

	// publish user.email_verified provisioning event
	provPayload, _ := json.Marshal(map[string]interface{}{"user_id": u.ID})
	if err := s.publisher.Publish(ctx, event.Event{Name: event.UserEmailVerified, Payload: provPayload}); err != nil {
		logger.Log.WithError(err).WithFields(map[string]interface{}{"user_id": u.ID}).Warn("failed to publish user.email_verified")
	}

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

	// publish verification_sent audit event
	payload, _ := json.Marshal(map[string]interface{}{"user_id": u.ID, "email": u.Email})
	if err := s.publisher.Publish(ctx, event.Event{Name: event.VerificationSent, Payload: payload}); err != nil {
		logger.Log.WithError(err).WithFields(map[string]interface{}{"user_id": u.ID}).Warn("failed to publish audit.verification_sent")
	}

	return nil
}

// Login validates credentials, resolves or creates the device, creates a stable session,
// and issues the first token in the rotation chain.
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
		payload, _ := json.Marshal(map[string]interface{}{
			"email":      req.Email,
			"ip_address": req.IPAddress,
			"user_agent": req.UserAgent,
		})
		if pubErr := s.publisher.Publish(ctx, event.Event{Name: event.LoginFailed, Payload: payload}); pubErr != nil {
			logger.Log.WithError(pubErr).Warn("failed to publish audit.login_failed")
		}
		return "", "", errs.ErrInvalidCredentials
	}

	if u.EmailVerifiedAt == nil {
		return "", "", errs.ErrEmailNotVerified
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
	os := req.OS

	// Step 2: Resolve or create device
	fingerprint := deviceName + "|" + userAgent
	isNewDevice := false
	var deviceID *int64

	if s.userDeviceRepo != nil {
		d, err := s.userDeviceRepo.GetByFingerprint(ctx, u.ID, fingerprint)
		if err != nil {
			if errors.Is(err, errs.ErrDeviceNotFound) {
				// New device — create record
				newD := &userdevice.Device{
					UserID:      u.ID,
					Fingerprint: fingerprint,
					DeviceName:  deviceName,
					Browser:     userAgent,
					OS:          os,
					IP:          ipAddress,
					FirstSeenAt: time.Now(),
					LastSeenAt:  time.Now(),
				}
				if createErr := s.userDeviceRepo.Create(ctx, newD); createErr != nil {
					logger.Log.WithError(createErr).WithFields(map[string]interface{}{"user_id": u.ID}).Warn("failed to create user device record")
				} else {
					isNewDevice = true
					deviceID = &newD.ID
				}
			} else {
				logger.Log.WithError(err).WithFields(map[string]interface{}{"user_id": u.ID}).Warn("failed to lookup user device")
			}
		} else {
			// Existing device — update last seen
			if updateErr := s.userDeviceRepo.UpdateLastSeen(ctx, d.ID, time.Now()); updateErr != nil {
				logger.Log.WithError(updateErr).WithFields(map[string]interface{}{"device_id": d.ID}).Warn("failed to update device last_seen_at")
			}
			deviceID = &d.ID
		}
	}

	// Step 3: Create stable session
	now := time.Now()
	session := &usersession.UserSession{
		UserID:         u.ID,
		DeviceID:       deviceID,
		Status:         usersession.SessionStatusActive,
		CreatedAt:      now,
		LastActivityAt: now,
		ExpiresAt:      now.Add(s.refreshTokenTTL),
	}
	if err = s.userSessionRepo.Create(ctx, session); err != nil {
		return "", "", fmt.Errorf("failed to create user session: %w", err)
	}

	// Step 4: Create first refresh token in chain
	refreshTokenStr, err := generateRefreshToken()
	if err != nil {
		return "", "", fmt.Errorf("failed to generate refresh token: %w", err)
	}

	rt := &token.RefreshToken{
		SessionID: session.ID,
		UserID:    u.ID,
		TokenHash: HashRefreshToken(refreshTokenStr),
		Status:    token.RefreshTokenStatusActive,
		CreatedAt: now,
		ExpiresAt: now.Add(s.refreshTokenTTL),
		// ParentTokenID is nil — first token in chain
	}
	if err = s.refreshTokenRepo.Create(ctx, rt); err != nil {
		return "", "", fmt.Errorf("failed to save refresh token: %w", err)
	}

	// Step 5: Generate access token
	accessToken, err := s.tokenManager.GenerateWithRole(u.ID, u.RoleName, u.TokenVersion)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate access token: %w", err)
	}

	// Publish login_success audit event
	loginSuccessPayload, _ := json.Marshal(map[string]interface{}{
		"user_id":    u.ID,
		"ip_address": ipAddress,
		"user_agent": userAgent,
		"device":     deviceName,
		"new_device": isNewDevice,
	})
	if pubErr := s.publisher.Publish(ctx, event.Event{Name: event.LoginSuccess, Payload: loginSuccessPayload}); pubErr != nil {
		logger.Log.WithError(pubErr).WithFields(map[string]interface{}{"user_id": u.ID}).Warn("failed to publish audit.login_success")
	}

	// If it's a new device, send login notification email
	if isNewDevice {
		htmlContent, textContent, tmplErr := mailer.RenderTemplate("login_notification", map[string]interface{}{
			"AppName":  s.appName,
			"Time":     now.Format(time.RFC3339),
			"IP":       ipAddress,
			"Device":   deviceName,
			"Browser":  userAgent,
			"Platform": os,
		})
		if tmplErr != nil {
			logger.Log.WithError(tmplErr).WithFields(map[string]interface{}{"user_id": u.ID}).Warn("failed to render login notification template")
		} else {
			emailPayload, _ := json.Marshal(map[string]interface{}{
				"to":      []string{u.Email},
				"subject": "New sign-in to your account",
				"html":    htmlContent,
				"text":    textContent,
			})
			if pubErr := s.publisher.Publish(ctx, event.Event{Name: event.EmailSend, Payload: emailPayload}); pubErr != nil {
				logger.Log.WithError(pubErr).WithFields(map[string]interface{}{"user_id": u.ID}).Warn("failed to publish email.send to outbox")
			}
		}
	}

	return accessToken, refreshTokenStr, nil
}

// Refresh validates a refresh token, detects reuse, rotates the token, and issues a new access token.
func (s *authService) Refresh(ctx context.Context, refreshTokenStr string, userAgent string, ipAddress string) (string, string, error) {
	if refreshTokenStr == "" {
		return "", "", errs.ErrInvalidRefreshToken
	}

	// Step 1: look up by hash
	hash := token.HashRefreshToken(refreshTokenStr)
	rt, err := s.refreshTokenRepo.GetByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, errs.ErrRefreshTokenNotFound) {
			return "", "", errs.ErrInvalidRefreshToken
		}
		return "", "", err
	}

	// Step 2: check effective status
	switch rt.EffectiveStatus() {
	case token.RefreshTokenStatusExpired:
		return "", "", errs.ErrInvalidRefreshToken

	case token.RefreshTokenStatusRevoked:
		// Reuse detection: revoke entire session (not all user sessions)
		s.handleReuseDetection(ctx, rt, ipAddress, userAgent)
		return "", "", errs.ErrInvalidRefreshToken
	}

	// Step 3: load and validate session
	session, err := s.userSessionRepo.GetByID(ctx, rt.SessionID)
	if err != nil {
		if errors.Is(err, errs.ErrSessionNotFound) {
			return "", "", errs.ErrInvalidRefreshToken
		}
		return "", "", err
	}
	if session.Status == usersession.SessionStatusRevoked || session.Status == usersession.SessionStatusExpired {
		return "", "", errs.ErrInvalidRefreshToken
	}

	// Step 4: look up user
	u, err := s.userRepo.GetByID(ctx, rt.UserID)
	if err != nil {
		return "", "", fmt.Errorf("failed to fetch user for refresh: %w", err)
	}
	if u.DeactivatedAt != nil {
		return "", "", errs.ErrUserDeactivated
	}

	// Step 5: generate new token string before rotation
	newRefreshTokenStr, err := generateRefreshToken()
	if err != nil {
		return "", "", fmt.Errorf("failed to generate refresh token: %w", err)
	}

	now := time.Now()
	parentID := rt.ID
	newRT := &token.RefreshToken{
		SessionID:     session.ID,
		UserID:        u.ID,
		TokenHash:     HashRefreshToken(newRefreshTokenStr),
		ParentTokenID: &parentID,
		Status:        token.RefreshTokenStatusActive,
		CreatedAt:     now,
		ExpiresAt:     now.Add(s.refreshTokenTTL),
	}

	// Step 5: atomic rotation
	if err := s.refreshTokenRepo.Rotate(ctx, rt.ID, newRT); err != nil {
		return "", "", fmt.Errorf("failed to rotate refresh token: %w", err)
	}

	// Step 6: update session activity
	if err := s.userSessionRepo.UpdateLastActivity(ctx, session.ID, now); err != nil {
		logger.Log.WithError(err).WithFields(map[string]interface{}{"session_id": session.ID}).Warn("failed to update session last_activity_at")
	}

	// publish refresh_rotated
	rotatedPayload, _ := json.Marshal(map[string]interface{}{
		"user_id":    rt.UserID,
		"ip_address": ipAddress,
		"user_agent": userAgent,
	})
	if pubErr := s.publisher.Publish(ctx, event.Event{Name: event.RefreshRotated, Payload: rotatedPayload}); pubErr != nil {
		logger.Log.WithError(pubErr).WithFields(map[string]interface{}{"user_id": rt.UserID}).Warn("failed to publish audit.refresh_rotated")
	}

	// Step 7: generate access token
	accessToken, err := s.tokenManager.GenerateWithRole(u.ID, u.RoleName, u.TokenVersion)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate access token: %w", err)
	}

	return accessToken, newRefreshTokenStr, nil
}

// handleReuseDetection revokes the compromised session (session-scoped, not user-scoped)
// and publishes a security audit event.
func (s *authService) handleReuseDetection(ctx context.Context, rt *token.RefreshToken, ipAddress, userAgent string) {
	if err := s.refreshTokenRepo.RevokeSessionTokens(ctx, rt.SessionID); err != nil {
		logger.Log.WithError(err).WithFields(map[string]interface{}{"session_id": rt.SessionID}).Warn("failed to revoke session tokens during reuse detection")
	}
	if err := s.userSessionRepo.Terminate(ctx, rt.SessionID); err != nil {
		logger.Log.WithError(err).WithFields(map[string]interface{}{"session_id": rt.SessionID}).Warn("failed to terminate session during reuse detection")
	}

	reusePayload, _ := json.Marshal(map[string]interface{}{
		"user_id":    rt.UserID,
		"session_id": rt.SessionID,
		"ip_address": ipAddress,
		"user_agent": userAgent,
		"token_hash": rt.TokenHash,
	})
	if pubErr := s.publisher.Publish(ctx, event.Event{Name: event.RefreshTokenReuseDetected, Payload: reusePayload}); pubErr != nil {
		logger.Log.WithError(pubErr).WithFields(map[string]interface{}{"user_id": rt.UserID}).Warn("failed to publish audit.refresh_token_reuse_detected")
	}

	// Notify user by email best-effort
	if u, err := s.userRepo.GetByID(ctx, rt.UserID); err == nil {
		go func() {
			msg := domainmailer.Message{
				To:      []string{u.Email},
				Subject: "Security alert: possible account compromise",
				Text:    "We detected the reuse of a previously revoked refresh token for your account. The affected session has been revoked. If this wasn't you, please change your password and review your active sessions.",
			}
			if sendErr := s.mailer.Send(context.Background(), msg); sendErr != nil {
				logger.Log.WithError(sendErr).WithFields(map[string]interface{}{"user_id": rt.UserID}).Warn("failed to send refresh-token-reuse notification email")
			}
		}()
	}
}

// Logout revokes the session and all its associated tokens.
func (s *authService) Logout(ctx context.Context, refreshTokenStr string) error {
	if refreshTokenStr == "" {
		return errs.ErrInvalidRefreshToken
	}

	hash := token.HashRefreshToken(refreshTokenStr)
	rt, err := s.refreshTokenRepo.GetByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, errs.ErrRefreshTokenNotFound) {
			return errs.ErrInvalidRefreshToken
		}
		return err
	}

	session, err := s.userSessionRepo.GetByID(ctx, rt.SessionID)
	if err != nil {
		if errors.Is(err, errs.ErrSessionNotFound) {
			return errs.ErrInvalidRefreshToken
		}
		return err
	}

	if err := s.userSessionRepo.Terminate(ctx, session.ID); err != nil {
		return fmt.Errorf("failed to terminate session: %w", err)
	}

	if err := s.refreshTokenRepo.RevokeSessionTokens(ctx, session.ID); err != nil {
		return fmt.Errorf("failed to revoke session tokens: %w", err)
	}

	// publish audit event
	logoutPayload, _ := json.Marshal(map[string]interface{}{
		"user_id":    rt.UserID,
		"session_id": session.ID,
	})
	if pubErr := s.publisher.Publish(ctx, event.Event{Name: event.Logout, Payload: logoutPayload}); pubErr != nil {
		logger.Log.WithError(pubErr).WithFields(map[string]interface{}{"user_id": rt.UserID}).Warn("failed to publish audit.logout")
	}

	return nil
}

// LogoutAll revokes all sessions and refresh tokens for the given user.
func (s *authService) LogoutAll(ctx context.Context, userID int64) error {
	if err := s.userSessionRepo.TerminateAll(ctx, userID); err != nil {
		return err
	}

	if err := s.refreshTokenRepo.RevokeAllByUserID(ctx, userID); err != nil {
		return err
	}

	if err := s.userRepo.IncrementTokenVersion(ctx, userID); err != nil {
		return err
	}

	if s.tokenVersionCache != nil {
		if err := s.tokenVersionCache.Delete(ctx, userID); err != nil {
			logger.Log.WithError(err).WithField("user_id", userID).Warn("failed to delete token version from cache")
		}
	}

	logoutAllPayload, _ := json.Marshal(map[string]interface{}{"user_id": userID})
	if pubErr := s.publisher.Publish(ctx, event.Event{Name: event.LogoutAll, Payload: logoutAllPayload}); pubErr != nil {
		logger.Log.WithError(pubErr).WithFields(map[string]interface{}{"user_id": userID}).Warn("failed to publish audit.logout_all")
	}

	return nil
}

// DeactivateUser marks a user as deactivated and revokes all sessions and refresh tokens.
func (s *authService) DeactivateUser(ctx context.Context, userID int64) error {
	now := time.Now()
	if err := s.userRepo.SetDeactivatedAt(ctx, userID, &now); err != nil {
		return err
	}

	if err := s.userSessionRepo.TerminateAll(ctx, userID); err != nil {
		return err
	}

	if err := s.refreshTokenRepo.RevokeAllByUserID(ctx, userID); err != nil {
		return err
	}

	if err := s.userRepo.IncrementTokenVersion(ctx, userID); err != nil {
		return err
	}

	if s.tokenVersionCache != nil {
		if err := s.tokenVersionCache.Delete(ctx, userID); err != nil {
			logger.Log.WithError(err).WithField("user_id", userID).Warn("failed to delete token version from cache")
		}
	}

	deactivatedPayload, _ := json.Marshal(map[string]interface{}{"user_id": userID})
	if pubErr := s.publisher.Publish(ctx, event.Event{Name: event.AccountDeactivated, Payload: deactivatedPayload}); pubErr != nil {
		logger.Log.WithError(pubErr).WithFields(map[string]interface{}{"user_id": userID}).Warn("failed to publish audit.account_deactivated")
	}

	return nil
}

// ReactivateUser clears the deactivated timestamp to re-enable the account.
func (s *authService) ReactivateUser(ctx context.Context, userID int64) error {
	if err := s.userRepo.SetDeactivatedAt(ctx, userID, nil); err != nil {
		return err
	}

	if s.tokenVersionCache != nil {
		if err := s.tokenVersionCache.Delete(ctx, userID); err != nil {
			logger.Log.WithError(err).WithField("user_id", userID).Warn("failed to delete token version from cache")
		}
	}

	activatedPayload, _ := json.Marshal(map[string]interface{}{"user_id": userID})
	if pubErr := s.publisher.Publish(ctx, event.Event{Name: event.AccountActivated, Payload: activatedPayload}); pubErr != nil {
		logger.Log.WithError(pubErr).WithFields(map[string]interface{}{"user_id": userID}).Warn("failed to publish audit.account_activated")
	}
	return nil
}

// ListSessions returns active sessions with device info for the given user.
func (s *authService) ListSessions(ctx context.Context, userID int64) ([]*usersession.SessionView, error) {
	return s.userSessionRepo.ListActiveByUserID(ctx, userID)
}

// RevokeSession terminates a specific session and revokes all its tokens.
// Regular users can only revoke their own sessions; admins can revoke any.
func (s *authService) RevokeSession(ctx context.Context, sessionID int64, userID int64, userRole string) error {
	session, err := s.userSessionRepo.GetByID(ctx, sessionID)
	if err != nil {
		return err
	}

	if userRole != "admin" && session.UserID != userID {
		return errs.ErrForbidden
	}

	if err := s.userSessionRepo.Terminate(ctx, sessionID); err != nil {
		return err
	}

	if err := s.refreshTokenRepo.RevokeSessionTokens(ctx, sessionID); err != nil {
		return err
	}

	return nil
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

	if err := s.userRepo.IncrementTokenVersion(ctx, userID); err != nil {
		return fmt.Errorf("failed to increment token version: %w", err)
	}

	if s.tokenVersionCache != nil {
		if err := s.tokenVersionCache.Delete(ctx, userID); err != nil {
			logger.Log.WithError(err).WithField("user_id", userID).Warn("failed to delete token version from cache")
		}
	}

	if err := s.userSessionRepo.TerminateAll(ctx, userID); err != nil {
		return fmt.Errorf("failed to terminate all sessions: %w", err)
	}

	if err := s.refreshTokenRepo.RevokeAllByUserID(ctx, userID); err != nil {
		return fmt.Errorf("failed to revoke refresh tokens: %w", err)
	}

	payload, _ := json.Marshal(map[string]interface{}{"user_id": userID})
	if pubErr := s.publisher.Publish(ctx, event.Event{Name: event.PasswordChanged, Payload: payload}); pubErr != nil {
		logger.Log.WithError(pubErr).WithFields(map[string]interface{}{"user_id": userID}).Warn("failed to publish audit.password_changed")
	}

	return nil
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

func (s *authService) sendVerificationEmail(ctx context.Context, email, verificationToken string) error {
	verificationURL := strings.TrimRight(s.appURL, "/") + "/auth/verify-email?token=" + verificationToken

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
