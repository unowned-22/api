package auth

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	domainevent "github.com/unowned-22/api/internal/domain/event"
	domainmailer "github.com/unowned-22/api/internal/domain/mailer"
	domainRole "github.com/unowned-22/api/internal/domain/role"
	domainToken "github.com/unowned-22/api/internal/domain/token"
	domainUser "github.com/unowned-22/api/internal/domain/user"
	userdevice "github.com/unowned-22/api/internal/domain/userdevice"
	"github.com/unowned-22/api/internal/domain/usersession"
	"github.com/unowned-22/api/internal/errs"
)

// ── mock: EventPublisher ─────────────────────────────────────────────────────

type mockEventPublisher struct {
	events []domainevent.Event
}

func (m *mockEventPublisher) Publish(ctx context.Context, event domainevent.Event) error {
	m.events = append(m.events, event)
	return nil
}

func (m *mockEventPublisher) PublishTx(ctx context.Context, tx pgx.Tx, event domainevent.Event) error {
	return m.Publish(ctx, event)
}

func (m *mockEventPublisher) Close() error { return nil }

type mockMailer struct {
	sendErr error
	lastMsg domainmailer.Message
}

func (m *mockMailer) Send(ctx context.Context, msg domainmailer.Message) error {
	m.lastMsg = msg
	return m.sendErr
}

// mockUserDeviceRepo simulates absence of existing devices (so Login treats as new)
type mockUserDeviceRepo struct {
	created *userdevice.Device
}

func (m *mockUserDeviceRepo) GetByUnique(ctx context.Context, userID int64, fingerprint, browser, country string) (*userdevice.Device, error) {
	return nil, nil
}

func (m *mockUserDeviceRepo) Create(ctx context.Context, d *userdevice.Device) error {
	m.created = d
	d.ID = 1
	return nil
}

func (m *mockUserDeviceRepo) UpdateLastSeen(ctx context.Context, id int64, t time.Time) error {
	return nil
}

// ── mock: UserRepository ─────────────────────────────────────────────────────

type mockUserRepo struct {
	users map[string]*domainUser.User
	idMap map[int64]*domainUser.User
	seq   int64
}

func newMockUserRepo() *mockUserRepo {
	return &mockUserRepo{
		users: make(map[string]*domainUser.User),
		idMap: make(map[int64]*domainUser.User),
	}
}

func (m *mockUserRepo) Create(ctx context.Context, u *domainUser.User) error {
	if _, ok := m.users[u.Email]; ok {
		return errs.ErrUserAlreadyExists
	}
	m.seq++
	u.ID = m.seq
	u.TokenVersion = 1
	if u.RoleID == 2 {
		u.RoleName = "user"
	} else if u.RoleID == 1 {
		u.RoleName = "admin"
	}
	cp := *u
	m.users[u.Email] = &cp
	m.idMap[u.ID] = &cp
	return nil
}

func (m *mockUserRepo) CreateTx(ctx context.Context, tx pgx.Tx, u *domainUser.User) error {
	return m.Create(ctx, u)
}

func (m *mockUserRepo) GetByEmail(ctx context.Context, email string) (*domainUser.User, error) {
	u, ok := m.users[email]
	if !ok {
		return nil, errs.ErrUserNotFound
	}
	return u, nil
}

func (m *mockUserRepo) GetByID(ctx context.Context, id int64) (*domainUser.User, error) {
	u, ok := m.idMap[id]
	if !ok {
		return nil, errs.ErrUserNotFound
	}
	return u, nil
}

func (m *mockUserRepo) SetVerificationToken(ctx context.Context, userID int64, token string, expiresAt time.Time) error {
	u, ok := m.idMap[userID]
	if !ok {
		return errs.ErrUserNotFound
	}
	u.VerificationToken = &token
	u.VerificationTokenExpiresAt = &expiresAt
	return nil
}

func (m *mockUserRepo) SetVerificationTokenTx(ctx context.Context, tx pgx.Tx, userID int64, token string, expiresAt time.Time) error {
	return m.SetVerificationToken(ctx, userID, token, expiresAt)
}

func (m *mockUserRepo) GetByVerificationToken(ctx context.Context, token string) (*domainUser.User, error) {
	for _, u := range m.users {
		if u.VerificationToken != nil && *u.VerificationToken == token {
			return u, nil
		}
	}
	return nil, errs.ErrVerificationTokenInvalid
}

func (m *mockUserRepo) MarkEmailVerified(ctx context.Context, userID int64) error {
	u, ok := m.idMap[userID]
	if !ok {
		return errs.ErrUserNotFound
	}
	now := time.Now()
	u.EmailVerifiedAt = &now
	u.VerificationToken = nil
	u.VerificationTokenExpiresAt = nil
	return nil
}

func (m *mockUserRepo) UpdatePassword(ctx context.Context, userID int64, hashedPassword string) error {
	u, ok := m.idMap[userID]
	if !ok {
		return errs.ErrUserNotFound
	}
	u.Password = hashedPassword
	return nil
}

func (m *mockUserRepo) IncrementTokenVersion(ctx context.Context, userID int64) error {
	u, ok := m.idMap[userID]
	if !ok {
		return errs.ErrUserNotFound
	}
	u.TokenVersion++
	return nil
}

func (m *mockUserRepo) SetDeactivatedAt(ctx context.Context, userID int64, t *time.Time) error {
	u, ok := m.idMap[userID]
	if !ok {
		return errs.ErrUserNotFound
	}
	u.DeactivatedAt = t
	return nil
}

func (m *mockUserRepo) UpdateProfile(ctx context.Context, userID int64, fullName, username, phone string) error {
	u, ok := m.idMap[userID]
	if !ok {
		return errs.ErrUserNotFound
	}
	// ensure username uniqueness
	for _, other := range m.idMap {
		if other.ID != userID && other.Username == username {
			return errs.ErrUsernameAlreadyExists
		}
	}
	u.FullName = fullName
	u.Username = username
	u.Phone = phone
	return nil
}

func (m *mockUserRepo) UpdateAvatar(ctx context.Context, userID int64, avatarURL string) error {
	u, ok := m.idMap[userID]
	if !ok {
		return errs.ErrUserNotFound
	}
	u.AvatarURL = avatarURL
	return nil
}

func (m *mockUserRepo) UpdateCover(ctx context.Context, userID int64, coverURL string) error {
	u, ok := m.idMap[userID]
	if !ok {
		return errs.ErrUserNotFound
	}
	u.CoverURL = coverURL
	return nil
}

func (m *mockUserRepo) List(ctx context.Context, offset int, limit int) ([]*domainUser.User, error) {
	var out []*domainUser.User
	for _, u := range m.idMap {
		out = append(out, u)
	}
	// simple stable order by id (not guaranteed) but ok for tests
	if offset >= len(out) {
		return []*domainUser.User{}, nil
	}
	end := offset + limit
	if end > len(out) {
		end = len(out)
	}
	return out[offset:end], nil
}

func (m *mockUserRepo) Count(ctx context.Context) (int64, error) {
	return int64(len(m.idMap)), nil
}

// ── mock: RefreshTokenRepository ─────────────────────────────────────────────

type mockRefreshTokenRepo struct {
	tokens map[string]*domainToken.RefreshToken
	seq    int64
}

func newMockRefreshTokenRepo() *mockRefreshTokenRepo {
	return &mockRefreshTokenRepo{tokens: make(map[string]*domainToken.RefreshToken)}
}

func (m *mockRefreshTokenRepo) CreateRefreshToken(ctx context.Context, t *domainToken.RefreshToken) error {
	m.seq++
	t.ID = m.seq
	cp := *t
	m.tokens[t.TokenHash] = &cp
	return nil
}

func (m *mockRefreshTokenRepo) GetByToken(ctx context.Context, tokenStr string) (*domainToken.RefreshToken, error) {
	hash := domainToken.HashRefreshToken(tokenStr)
	t, ok := m.tokens[hash]
	if !ok {
		return nil, errs.ErrRefreshTokenNotFound
	}
	return t, nil
}

func (m *mockRefreshTokenRepo) RevokeRefreshToken(ctx context.Context, tokenStr string) error {
	hash := domainToken.HashRefreshToken(tokenStr)
	t, ok := m.tokens[hash]
	if !ok {
		return errs.ErrRefreshTokenNotFound
	}
	t.Status = domainToken.RefreshTokenStatusRevoked
	return nil
}

func (m *mockRefreshTokenRepo) DeleteExpired(ctx context.Context) error {
	for k, v := range m.tokens {
		if v.ExpiresAt.Before(time.Now()) {
			delete(m.tokens, k)
		}
	}
	return nil
}

func (m *mockRefreshTokenRepo) RevokeAllByUserID(ctx context.Context, userID int64) error {
	for _, t := range m.tokens {
		if t.UserID == userID {
			t.Status = domainToken.RefreshTokenStatusRevoked
		}
	}
	return nil
}

// ── mock: UserSessionRepository ──────────────────────────────────────────────

type mockUserSessionRepo struct {
	sessions         map[int64]*usersession.UserSession
	byRefreshTokenID map[int64]*usersession.UserSession
	seq              int64
}

func newMockUserSessionRepo() *mockUserSessionRepo {
	return &mockUserSessionRepo{
		sessions:         make(map[int64]*usersession.UserSession),
		byRefreshTokenID: make(map[int64]*usersession.UserSession),
	}
}

func (m *mockUserSessionRepo) Create(ctx context.Context, session *usersession.UserSession) error {
	m.seq++
	session.ID = m.seq
	cp := *session
	m.sessions[session.ID] = &cp
	m.byRefreshTokenID[session.RefreshTokenID] = &cp
	return nil
}

func (m *mockUserSessionRepo) GetByID(ctx context.Context, id int64) (*usersession.UserSession, error) {
	session, ok := m.sessions[id]
	if !ok {
		return nil, errs.ErrSessionNotFound
	}
	return session, nil
}

func (m *mockUserSessionRepo) GetByRefreshTokenID(ctx context.Context, refreshTokenID int64) (*usersession.UserSession, error) {
	session, ok := m.byRefreshTokenID[refreshTokenID]
	if !ok {
		return nil, errs.ErrSessionNotFound
	}
	return session, nil
}

func (m *mockUserSessionRepo) ListActiveByUserID(ctx context.Context, userID int64) ([]*usersession.UserSession, error) {
	var sessions []*usersession.UserSession
	for _, session := range m.sessions {
		if session.UserID == userID && session.RevokedAt == nil {
			sessions = append(sessions, session)
		}
	}
	return sessions, nil
}

func (m *mockUserSessionRepo) Update(ctx context.Context, session *usersession.UserSession) error {
	existing, ok := m.sessions[session.ID]
	if !ok {
		return errs.ErrSessionNotFound
	}
	delete(m.byRefreshTokenID, existing.RefreshTokenID)
	cp := *session
	m.sessions[session.ID] = &cp
	m.byRefreshTokenID[session.RefreshTokenID] = &cp
	return nil
}

func (m *mockUserSessionRepo) Revoke(ctx context.Context, id int64) error {
	session, ok := m.sessions[id]
	if !ok {
		return errs.ErrSessionNotFound
	}
	now := time.Now()
	session.RevokedAt = &now
	return nil
}

func (m *mockUserSessionRepo) RevokeAllByUserID(ctx context.Context, userID int64) error {
	now := time.Now()
	for _, session := range m.sessions {
		if session.UserID == userID && session.RevokedAt == nil {
			session.RevokedAt = &now
		}
	}
	return nil
}

// ── mock: RoleRepository ─────────────────────────────────────────────────────

type mockRoleRepo struct {
	byName map[string]*domainRole.Role
	byID   map[int64]*domainRole.Role
}

func newMockRoleRepo() *mockRoleRepo {
	userRole := &domainRole.Role{ID: 2, Name: "user"}
	adminRole := &domainRole.Role{ID: 1, Name: "admin"}
	return &mockRoleRepo{
		byName: map[string]*domainRole.Role{"user": userRole, "admin": adminRole},
		byID:   map[int64]*domainRole.Role{2: userRole, 1: adminRole},
	}
}

func (m *mockRoleRepo) GetByID(ctx context.Context, id int64) (*domainRole.Role, error) {
	r, ok := m.byID[id]
	if !ok {
		return nil, errs.ErrRoleNotFound
	}
	return r, nil
}

func (m *mockRoleRepo) GetByName(ctx context.Context, name string) (*domainRole.Role, error) {
	r, ok := m.byName[name]
	if !ok {
		return nil, errs.ErrRoleNotFound
	}
	return r, nil
}

func (m *mockRoleRepo) List(ctx context.Context) ([]*domainRole.Role, error) {
	roles := make([]*domainRole.Role, 0, len(m.byName))
	for _, r := range m.byName {
		roles = append(roles, r)
	}
	return roles, nil
}

// ── mock: token.ManagerExtended ──────────────────────────────────────────────

type mockTokenManager struct{}

func (m *mockTokenManager) Generate(userID int64) (string, error) {
	return "mock-token-for-user", nil
}

func (m *mockTokenManager) Parse(tokenStr string) (int64, error) {
	if tokenStr == "mock-token-for-user" {
		return 1, nil
	}
	return 0, errors.New("invalid token")
}

func (m *mockTokenManager) GenerateWithRole(userID int64, role string, tokenVersion int) (string, error) {
	return "mock-token-for-user", nil
}

func (m *mockTokenManager) ParseWithRole(tokenStr string) (int64, string, int, error) {
	if tokenStr == "mock-token-for-user" {
		return 1, "user", 1, nil
	}
	return 0, "", 0, errors.New("invalid token")
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestAuthService(t *testing.T) {
	userRepo := newMockUserRepo()
	refreshTokenRepo := newMockRefreshTokenRepo()
	roleRepo := newMockRoleRepo()
	tm := &mockTokenManager{}
	mailer := &mockMailer{}
	publisher := &mockEventPublisher{}
	sessionRepo := newMockUserSessionRepo()
	srv := NewAuthService(userRepo, refreshTokenRepo, sessionRepo, nil, roleRepo, tm, mailer, publisher, 720*time.Hour, "http://localhost:3222", "App", nil, nil)

	ctx := context.Background()

	// 1. Register
	err := srv.Register(ctx, RegisterRequest{Email: "test@example.com", Password: "password123"})
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Force verify email for the mock user so login succeeds.
	u, err := userRepo.GetByEmail(ctx, "test@example.com")
	if err != nil {
		t.Fatalf("GetByEmail failed: %v", err)
	}
	now := time.Now()
	u.EmailVerifiedAt = &now

	// 2. Duplicate register
	err = srv.Register(ctx, RegisterRequest{Email: "test@example.com", Password: "password123"})
	if !errors.Is(err, errs.ErrUserAlreadyExists) {
		t.Errorf("expected ErrUserAlreadyExists, got %v", err)
	}

	// 3. Login success — token and refresh token returned
	accessToken, refreshToken, err := srv.Login(ctx, LoginRequest{Email: "test@example.com", Password: "password123"})
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	if accessToken != "mock-token-for-user" {
		t.Errorf("unexpected access token: %s", accessToken)
	}
	if refreshToken == "" {
		t.Errorf("expected non-empty refresh token")
	}
	sessions, err := srv.ListSessions(ctx, u.ID)
	if err != nil {
		t.Fatalf("ListSessions failed: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected one active session, got %d", len(sessions))
	}
	loginSession := sessions[0]
	originalLastUsedAt := loginSession.LastUsedAt

	// 4. Login invalid password
	_, _, err = srv.Login(ctx, LoginRequest{Email: "test@example.com", Password: "wrongpassword"})
	if !errors.Is(err, errs.ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}

	// 5. Login unknown user
	_, _, err = srv.Login(ctx, LoginRequest{Email: "nobody@example.com", Password: "password123"})
	if !errors.Is(err, errs.ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}

	// 6. Refresh success returns a new access token and a rotated refresh token.
	newAccessToken, newRefreshToken, err := srv.Refresh(ctx, refreshToken, "new-agent", "127.0.0.2")
	if err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}
	if newAccessToken != "mock-token-for-user" {
		t.Errorf("unexpected refreshed access token: %s", newAccessToken)
	}
	if newRefreshToken == "" || newRefreshToken == refreshToken {
		t.Errorf("expected a new refresh token")
	}
	newRT, err := refreshTokenRepo.GetByToken(ctx, newRefreshToken)
	if err != nil {
		t.Fatalf("GetByToken for rotated token failed: %v", err)
	}
	updatedSession, err := sessionRepo.GetByRefreshTokenID(ctx, newRT.ID)
	if err != nil {
		t.Fatalf("GetByRefreshTokenID for rotated session failed: %v", err)
	}
	if updatedSession.ID != loginSession.ID {
		t.Errorf("expected refresh to keep session id %d, got %d", loginSession.ID, updatedSession.ID)
	}
	if !updatedSession.LastUsedAt.After(originalLastUsedAt) {
		t.Errorf("expected last_used_at to be updated during refresh")
	}
	if updatedSession.UserAgent != "new-agent" {
		t.Errorf("expected user agent to update, got %q", updatedSession.UserAgent)
	}
	if updatedSession.IPAddress != "127.0.0.2" {
		t.Errorf("expected ip address to update, got %q", updatedSession.IPAddress)
	}

	// 7. Reusing the old refresh token must fail.
	_, _, err = srv.Refresh(ctx, refreshToken, "", "")
	if !errors.Is(err, errs.ErrInvalidRefreshToken) {
		t.Errorf("expected ErrInvalidRefreshToken for reused token, got %v", err)
	}

	// 8. Refresh with unknown token
	_, _, err = srv.Refresh(ctx, "non-existent-token", "", "")
	if !errors.Is(err, errs.ErrInvalidRefreshToken) {
		t.Errorf("expected ErrInvalidRefreshToken, got %v", err)
	}

	// 9. Logout revokes the active refresh token.
	if err = srv.Logout(ctx, newRefreshToken); err != nil {
		t.Fatalf("Logout failed: %v", err)
	}

	// 10. Refresh after logout (revoked)
	_, _, err = srv.Refresh(ctx, newRefreshToken, "", "")
	if !errors.Is(err, errs.ErrInvalidRefreshToken) {
		t.Errorf("expected ErrInvalidRefreshToken after logout, got %v", err)
	}

	// 11. Refresh expired token
	expiredTokenValue := "expired-token"
	expiredToken := &domainToken.RefreshToken{
		UserID:    1,
		TokenHash: domainToken.HashRefreshToken(expiredTokenValue),
		ExpiresAt: time.Now().Add(-1 * time.Hour),
		Status:    domainToken.RefreshTokenStatusActive,
	}
	_ = refreshTokenRepo.CreateRefreshToken(ctx, expiredToken)
	_, _, err = srv.Refresh(ctx, expiredTokenValue, "", "")
	if !errors.Is(err, errs.ErrInvalidRefreshToken) {
		t.Errorf("expected ErrInvalidRefreshToken for expired token, got %v", err)
	}
}

func TestRevokeSessionPreventsRefresh(t *testing.T) {
	userRepo := newMockUserRepo()
	refreshTokenRepo := newMockRefreshTokenRepo()
	sessionRepo := newMockUserSessionRepo()
	srv := NewAuthService(userRepo, refreshTokenRepo, sessionRepo, nil, newMockRoleRepo(), &mockTokenManager{}, &mockMailer{}, &mockEventPublisher{}, 720*time.Hour, "http://localhost:3222", "App", nil, nil)
	ctx := context.Background()

	if err := srv.Register(ctx, RegisterRequest{Email: "sessions@example.com", Password: "password123"}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	u, err := userRepo.GetByEmail(ctx, "sessions@example.com")
	if err != nil {
		t.Fatalf("GetByEmail failed: %v", err)
	}
	now := time.Now()
	u.EmailVerifiedAt = &now

	_, refreshToken, err := srv.Login(ctx, LoginRequest{Email: "sessions@example.com", Password: "password123", DeviceName: "Test Device"})
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	sessions, err := srv.ListSessions(ctx, u.ID)
	if err != nil {
		t.Fatalf("ListSessions failed: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected one active session, got %d", len(sessions))
	}

	if err := srv.RevokeSession(ctx, sessions[0].ID, u.ID, "user"); err != nil {
		t.Fatalf("RevokeSession failed: %v", err)
	}
	_, _, err = srv.Refresh(ctx, refreshToken, "", "")
	if !errors.Is(err, errs.ErrInvalidRefreshToken) {
		t.Errorf("expected ErrInvalidRefreshToken after session revoke, got %v", err)
	}

	activeSessions, err := srv.ListSessions(ctx, u.ID)
	if err != nil {
		t.Fatalf("ListSessions after revoke failed: %v", err)
	}
	if len(activeSessions) != 0 {
		t.Fatalf("expected no active sessions after revoke, got %d", len(activeSessions))
	}
}

func TestRegisterAssignsDefaultRole(t *testing.T) {
	userRepo := newMockUserRepo()
	mailer := &mockMailer{}
	publisher := &mockEventPublisher{}
	srv := NewAuthService(userRepo, newMockRefreshTokenRepo(), newMockUserSessionRepo(), nil, newMockRoleRepo(), &mockTokenManager{}, mailer, publisher, 720*time.Hour, "http://localhost:3222", "App", nil, nil)
	ctx := context.Background()

	if err := srv.Register(ctx, RegisterRequest{Email: "newuser@example.com", Password: "pass"}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	u, err := userRepo.GetByEmail(ctx, "newuser@example.com")
	if err != nil {
		t.Fatalf("GetByEmail failed: %v", err)
	}
	if u.RoleID != 2 {
		t.Errorf("expected RoleID=2 (user), got %d", u.RoleID)
	}
	if u.RoleName != "user" {
		t.Errorf("expected RoleName='user', got '%s'", u.RoleName)
	}
}

func TestDeactivateUserRevokesSessionsAndDeniesAuth(t *testing.T) {
	userRepo := newMockUserRepo()
	refreshTokenRepo := newMockRefreshTokenRepo()
	sessionRepo := newMockUserSessionRepo()
	roleRepo := newMockRoleRepo()
	tm := &mockTokenManager{}
	mailer := &mockMailer{}
	publisher := &mockEventPublisher{}
	srv := NewAuthService(userRepo, refreshTokenRepo, sessionRepo, nil, roleRepo, tm, mailer, publisher, 720*time.Hour, "http://localhost:3222", "App", nil, nil)

	ctx := context.Background()
	if err := srv.Register(ctx, RegisterRequest{Email: "disable@example.com", Password: "password123"}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	u, err := userRepo.GetByEmail(ctx, "disable@example.com")
	if err != nil {
		t.Fatalf("GetByEmail failed: %v", err)
	}
	now := time.Now()
	u.EmailVerifiedAt = &now

	_, refreshToken, err := srv.Login(ctx, LoginRequest{Email: "disable@example.com", Password: "password123"})
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	sessions, err := srv.ListSessions(ctx, u.ID)
	if err != nil {
		t.Fatalf("ListSessions failed: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected one active session, got %d", len(sessions))
	}

	// Deactivate user
	if err := srv.DeactivateUser(ctx, u.ID); err != nil {
		t.Fatalf("DeactivateUser failed: %v", err)
	}

	// Sessions should be revoked
	activeSessions, err := srv.ListSessions(ctx, u.ID)
	if err != nil {
		t.Fatalf("ListSessions after deactivate failed: %v", err)
	}
	if len(activeSessions) != 0 {
		t.Fatalf("expected no active sessions after deactivate, got %d", len(activeSessions))
	}

	// Login must be denied
	_, _, err = srv.Login(ctx, LoginRequest{Email: "disable@example.com", Password: "password123"})
	if !errors.Is(err, errs.ErrUserDeactivated) {
		t.Errorf("expected ErrUserDeactivated on login after deactivation, got %v", err)
	}

	// Refresh should be invalid because tokens were revoked
	_, _, err = srv.Refresh(ctx, refreshToken, "", "")
	if !errors.Is(err, errs.ErrInvalidRefreshToken) && !errors.Is(err, errs.ErrUserDeactivated) {
		t.Errorf("expected ErrInvalidRefreshToken or ErrUserDeactivated on refresh after deactivation, got %v", err)
	}
}

func TestReactivateUserAllowsLogin(t *testing.T) {
	userRepo := newMockUserRepo()
	refreshTokenRepo := newMockRefreshTokenRepo()
	sessionRepo := newMockUserSessionRepo()
	roleRepo := newMockRoleRepo()
	tm := &mockTokenManager{}
	mailer := &mockMailer{}
	publisher := &mockEventPublisher{}
	srv := NewAuthService(userRepo, refreshTokenRepo, sessionRepo, nil, roleRepo, tm, mailer, publisher, 720*time.Hour, "http://localhost:3222", "App", nil, nil)

	ctx := context.Background()
	if err := srv.Register(ctx, RegisterRequest{Email: "reactivate@example.com", Password: "password123"}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	u, err := userRepo.GetByEmail(ctx, "reactivate@example.com")
	if err != nil {
		t.Fatalf("GetByEmail failed: %v", err)
	}
	now := time.Now()
	u.EmailVerifiedAt = &now

	// Deactivate then Reactivate
	if err := srv.DeactivateUser(ctx, u.ID); err != nil {
		t.Fatalf("DeactivateUser failed: %v", err)
	}
	// ensure login is denied
	_, _, err = srv.Login(ctx, LoginRequest{Email: "reactivate@example.com", Password: "password123"})
	if !errors.Is(err, errs.ErrUserDeactivated) {
		t.Fatalf("expected ErrUserDeactivated after deactivate, got %v", err)
	}

	if err := srv.ReactivateUser(ctx, u.ID); err != nil {
		t.Fatalf("ReactivateUser failed: %v", err)
	}

	// login should succeed now
	_, _, err = srv.Login(ctx, LoginRequest{Email: "reactivate@example.com", Password: "password123"})
	if err != nil {
		t.Fatalf("expected login to succeed after reactivate, got %v", err)
	}
}

func TestLoginStoresRefreshTokenHashOnly(t *testing.T) {
	userRepo := newMockUserRepo()
	refreshTokenRepo := newMockRefreshTokenRepo()
	roleRepo := newMockRoleRepo()
	tm := &mockTokenManager{}
	mailer := &mockMailer{}
	publisher := &mockEventPublisher{}
	srv := NewAuthService(userRepo, refreshTokenRepo, newMockUserSessionRepo(), nil, roleRepo, tm, mailer, publisher, 720*time.Hour, "http://localhost:3222", "App", nil, nil)
	ctx := context.Background()

	if err := srv.Register(ctx, RegisterRequest{Email: "security@example.com", Password: "password123"}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Force verify email for the mock user so login succeeds.
	u, err := userRepo.GetByEmail(ctx, "security@example.com")
	if err != nil {
		t.Fatalf("GetByEmail failed: %v", err)
	}
	now := time.Now()
	u.EmailVerifiedAt = &now

	_, refreshToken, err := srv.Login(ctx, LoginRequest{Email: "security@example.com", Password: "password123"})
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	if refreshToken == "" {
		t.Fatal("expected refresh token")
	}

	hash := domainToken.HashRefreshToken(refreshToken)
	if _, ok := refreshTokenRepo.tokens[hash]; !ok {
		t.Fatalf("expected token hash stored in repository")
	}

	for key := range refreshTokenRepo.tokens {
		if key == refreshToken {
			t.Fatal("plain refresh token must not be stored")
		}
	}
}

func TestLoginSendsNewDeviceNotification(t *testing.T) {
	userRepo := newMockUserRepo()
	refreshTokenRepo := newMockRefreshTokenRepo()
	sessionRepo := newMockUserSessionRepo()
	roleRepo := newMockRoleRepo()
	tm := &mockTokenManager{}
	publisher := &mockEventPublisher{}
	mailer := &mockMailer{}
	deviceRepo := &mockUserDeviceRepo{}

	srv := NewAuthService(userRepo, refreshTokenRepo, sessionRepo, deviceRepo, roleRepo, tm, mailer, publisher, 720*time.Hour, "http://localhost:3222", "App", nil, nil)

	ctx := context.Background()
	if err := srv.Register(ctx, RegisterRequest{Email: "notify@example.com", Password: "password123"}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	u, _ := userRepo.GetByEmail(ctx, "notify@example.com")
	now := time.Now()
	u.EmailVerifiedAt = &now

	_, _, err := srv.Login(ctx, LoginRequest{Email: "notify@example.com", Password: "password123", DeviceName: "MyPhone", UserAgent: "UA/1.0", IPAddress: "1.2.3.4"})
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	if deviceRepo.created == nil {
		t.Fatalf("expected deviceRepo.Create to be called")
	}

	// wait briefly for asynchronous outbox publish
	waited := time.Now()
	for time.Since(waited) < 3*time.Second {
		if len(publisher.events) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	var found *domainevent.Event
	for i := range publisher.events {
		if publisher.events[i].Name == domainevent.EmailSend {
			found = &publisher.events[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("expected an outbox email.send event to be published, got events: %v", publisher.events)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(found.Payload, &payload); err != nil {
		t.Fatalf("failed to unmarshal event payload: %v", err)
	}
	tos, ok := payload["to"].([]interface{})
	if !ok || len(tos) == 0 || tos[0].(string) != "notify@example.com" {
		t.Fatalf("expected payload.to to contain notify@example.com, got %v", payload["to"])
	}
}
