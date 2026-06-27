package auth

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	auth2 "github.com/unowned-22/api/internal/auth"
	domainevent "github.com/unowned-22/api/internal/domain/event"
	domainmailer "github.com/unowned-22/api/internal/domain/mailer"
	domainRole "github.com/unowned-22/api/internal/domain/role"
	domainToken "github.com/unowned-22/api/internal/domain/token"
	domainUser "github.com/unowned-22/api/internal/domain/user"
	"github.com/unowned-22/api/internal/domain/userdevice"
	"github.com/unowned-22/api/internal/domain/usersession"
	"github.com/unowned-22/api/internal/errs"
)

// ── mock: EventPublisher ──────────────────────────────────────────────────────

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

// ── mock: Mailer ──────────────────────────────────────────────────────────────

type mockMailer struct {
	sendErr error
	lastMsg domainmailer.Message
}

func (m *mockMailer) Send(ctx context.Context, msg domainmailer.Message) error {
	m.lastMsg = msg
	return m.sendErr
}

// ── mock: UserDeviceRepository ────────────────────────────────────────────────

type mockUserDeviceRepo struct {
	devices map[string]*userdevice.Device // key: fingerprint
	seq     int64
	created *userdevice.Device
}

func newMockUserDeviceRepo() *mockUserDeviceRepo {
	return &mockUserDeviceRepo{devices: make(map[string]*userdevice.Device)}
}

func (m *mockUserDeviceRepo) GetByFingerprint(ctx context.Context, userID int64, fingerprint string) (*userdevice.Device, error) {
	d, ok := m.devices[fingerprint]
	if !ok {
		return nil, errs.ErrDeviceNotFound
	}
	return d, nil
}

func (m *mockUserDeviceRepo) Create(ctx context.Context, d *userdevice.Device) error {
	m.seq++
	d.ID = m.seq
	cp := *d
	m.devices[d.Fingerprint] = &cp
	m.created = &cp
	return nil
}

func (m *mockUserDeviceRepo) UpdateLastSeen(ctx context.Context, id int64, t time.Time) error {
	for _, d := range m.devices {
		if d.ID == id {
			d.LastSeenAt = t
		}
	}
	return nil
}

// ── mock: UserRepository ──────────────────────────────────────────────────────

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

func (m *mockUserRepo) GetByUsername(ctx context.Context, username string) (*domainUser.User, error) {
	for _, u := range m.idMap {
		if u.Username == username {
			return u, nil
		}
	}
	return nil, errs.ErrUserNotFound
}

// ── mock: RefreshTokenRepository ─────────────────────────────────────────────

type mockRefreshTokenRepo struct {
	tokens map[string]*domainToken.RefreshToken // key: token_hash
	seq    int64
}

func newMockRefreshTokenRepo() *mockRefreshTokenRepo {
	return &mockRefreshTokenRepo{tokens: make(map[string]*domainToken.RefreshToken)}
}

func (m *mockRefreshTokenRepo) Create(ctx context.Context, t *domainToken.RefreshToken) error {
	m.seq++
	t.ID = m.seq
	cp := *t
	m.tokens[t.TokenHash] = &cp
	return nil
}

func (m *mockRefreshTokenRepo) GetByHash(ctx context.Context, tokenHash string) (*domainToken.RefreshToken, error) {
	t, ok := m.tokens[tokenHash]
	if !ok {
		return nil, errs.ErrRefreshTokenNotFound
	}
	return t, nil
}

func (m *mockRefreshTokenRepo) Rotate(ctx context.Context, oldTokenID int64, newToken *domainToken.RefreshToken) error {
	// revoke old
	for _, t := range m.tokens {
		if t.ID == oldTokenID {
			t.Status = domainToken.RefreshTokenStatusRevoked
			t.ReplacedByTokenID = &newToken.ID
		}
	}
	// insert new
	return m.Create(ctx, newToken)
}

func (m *mockRefreshTokenRepo) Revoke(ctx context.Context, tokenID int64) error {
	for _, t := range m.tokens {
		if t.ID == tokenID {
			t.Status = domainToken.RefreshTokenStatusRevoked
			return nil
		}
	}
	return errs.ErrRefreshTokenNotFound
}

func (m *mockRefreshTokenRepo) RevokeSessionTokens(ctx context.Context, sessionID int64) error {
	for _, t := range m.tokens {
		if t.SessionID == sessionID {
			t.Status = domainToken.RefreshTokenStatusRevoked
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

func (m *mockRefreshTokenRepo) GetTokenChain(ctx context.Context, sessionID int64) ([]*domainToken.RefreshToken, error) {
	var chain []*domainToken.RefreshToken
	for _, t := range m.tokens {
		if t.SessionID == sessionID {
			chain = append(chain, t)
		}
	}
	return chain, nil
}

func (m *mockRefreshTokenRepo) DeleteExpired(ctx context.Context) error {
	for k, v := range m.tokens {
		if v.ExpiresAt.Before(time.Now()) {
			delete(m.tokens, k)
		}
	}
	return nil
}

// ── mock: UserSessionRepository ──────────────────────────────────────────────

type mockUserSessionRepo struct {
	sessions map[int64]*usersession.UserSession
	seq      int64
}

func newMockUserSessionRepo() *mockUserSessionRepo {
	return &mockUserSessionRepo{sessions: make(map[int64]*usersession.UserSession)}
}

func (m *mockUserSessionRepo) Create(ctx context.Context, s *usersession.UserSession) error {
	m.seq++
	s.ID = m.seq
	cp := *s
	m.sessions[s.ID] = &cp
	return nil
}

func (m *mockUserSessionRepo) GetByID(ctx context.Context, id int64) (*usersession.UserSession, error) {
	s, ok := m.sessions[id]
	if !ok {
		return nil, errs.ErrSessionNotFound
	}
	return s, nil
}

func (m *mockUserSessionRepo) ListActiveByUserID(ctx context.Context, userID int64) ([]*usersession.SessionView, error) {
	var out []*usersession.SessionView
	for _, s := range m.sessions {
		if s.UserID == userID &&
			s.Status == usersession.SessionStatusActive &&
			s.ExpiresAt.After(time.Now()) {
			out = append(out, &usersession.SessionView{UserSession: *s})
		}
	}
	return out, nil
}

func (m *mockUserSessionRepo) Terminate(ctx context.Context, id int64) error {
	s, ok := m.sessions[id]
	if !ok {
		return errs.ErrSessionNotFound
	}
	s.Status = usersession.SessionStatusRevoked
	return nil
}

func (m *mockUserSessionRepo) TerminateAll(ctx context.Context, userID int64) error {
	for _, s := range m.sessions {
		if s.UserID == userID && s.Status != usersession.SessionStatusRevoked {
			s.Status = usersession.SessionStatusRevoked
		}
	}
	return nil
}

func (m *mockUserSessionRepo) UpdateLastActivity(ctx context.Context, id int64, t time.Time) error {
	s, ok := m.sessions[id]
	if !ok {
		return errs.ErrSessionNotFound
	}
	s.LastActivityAt = t
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

// ── helpers ───────────────────────────────────────────────────────────────────

// newTestService builds a fully wired authService for tests.
func newTestService(
	userRepo *mockUserRepo,
	rtRepo *mockRefreshTokenRepo,
	sessionRepo *mockUserSessionRepo,
	deviceRepo userdevice.Repository,
) auth2.AuthService {
	return auth2.NewAuthService(
		userRepo,
		rtRepo,
		sessionRepo,
		deviceRepo,
		newMockRoleRepo(),
		&mockTokenManager{},
		&mockMailer{},
		&mockEventPublisher{},
		720*time.Hour,
		"http://localhost:3222",
		"App",
		nil,
		nil, // pool: nil — tests use mock repos, no real transaction needed
	)
}

// registerAndVerify registers a user and force-marks the email as verified.
func registerAndVerify(t *testing.T, srv auth2.AuthService, userRepo *mockUserRepo, email, password string) *domainUser.User {
	t.Helper()
	ctx := context.Background()
	if err := srv.Register(ctx, auth2.RegisterRequest{Email: email, Password: password}); err != nil {
		t.Fatalf("Register(%s) failed: %v", email, err)
	}
	u, err := userRepo.GetByEmail(ctx, email)
	if err != nil {
		t.Fatalf("GetByEmail(%s) failed: %v", email, err)
	}
	now := time.Now()
	u.EmailVerifiedAt = &now
	return u
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestAuthService(t *testing.T) {
	userRepo := newMockUserRepo()
	rtRepo := newMockRefreshTokenRepo()
	sessionRepo := newMockUserSessionRepo()
	srv := newTestService(userRepo, rtRepo, sessionRepo, nil)
	ctx := context.Background()

	// 1. Register
	if err := srv.Register(ctx, auth2.RegisterRequest{Email: "test@example.com", Password: "password123"}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	u, _ := userRepo.GetByEmail(ctx, "test@example.com")
	now := time.Now()
	u.EmailVerifiedAt = &now

	// 2. Duplicate register
	if err := srv.Register(ctx, auth2.RegisterRequest{Email: "test@example.com", Password: "password123"}); !errors.Is(err, errs.ErrUserAlreadyExists) {
		t.Errorf("expected ErrUserAlreadyExists, got %v", err)
	}

	// 3. Login success
	accessToken, refreshToken, err := srv.Login(ctx, auth2.LoginRequest{Email: "test@example.com", Password: "password123"})
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	if accessToken != "mock-token-for-user" {
		t.Errorf("unexpected access token: %s", accessToken)
	}
	if refreshToken == "" {
		t.Error("expected non-empty refresh token")
	}

	sessions, err := srv.ListSessions(ctx, u.ID)
	if err != nil {
		t.Fatalf("ListSessions failed: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected one active session, got %d", len(sessions))
	}
	loginSessionID := sessions[0].ID
	originalLastActivity := sessions[0].LastActivityAt

	// 4. Invalid password
	if _, _, err = srv.Login(ctx, auth2.LoginRequest{Email: "test@example.com", Password: "wrongpassword"}); !errors.Is(err, errs.ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}

	// 5. Unknown user
	if _, _, err = srv.Login(ctx, auth2.LoginRequest{Email: "nobody@example.com", Password: "password123"}); !errors.Is(err, errs.ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}

	// 6. Refresh success → new token, session unchanged
	time.Sleep(1 * time.Millisecond) // ensure last_activity_at moves
	newAccessToken, newRefreshToken, err := srv.Refresh(ctx, refreshToken, "new-agent", "127.0.0.2")
	if err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}
	if newAccessToken != "mock-token-for-user" {
		t.Errorf("unexpected refreshed access token: %s", newAccessToken)
	}
	if newRefreshToken == "" || newRefreshToken == refreshToken {
		t.Error("expected a new distinct refresh token")
	}

	// Verify new token hash is stored
	newHash := domainToken.HashRefreshToken(newRefreshToken)
	newRT, ok := rtRepo.tokens[newHash]
	if !ok {
		t.Fatal("new rotated token not found in repo")
	}

	// Session must be the same stable session
	updatedSession, err := sessionRepo.GetByID(ctx, loginSessionID)
	if err != nil {
		t.Fatalf("GetByID for session failed: %v", err)
	}
	if newRT.SessionID != loginSessionID {
		t.Errorf("expected new token to belong to session %d, got %d", loginSessionID, newRT.SessionID)
	}
	if !updatedSession.LastActivityAt.After(originalLastActivity) {
		t.Error("expected last_activity_at to advance during refresh")
	}

	// 7. Reusing the old (now-revoked) refresh token must trigger reuse detection
	if _, _, err = srv.Refresh(ctx, refreshToken, "", ""); !errors.Is(err, errs.ErrInvalidRefreshToken) {
		t.Errorf("expected ErrInvalidRefreshToken for reused token, got %v", err)
	}

	// 8. Unknown token
	if _, _, err = srv.Refresh(ctx, "non-existent-token", "", ""); !errors.Is(err, errs.ErrInvalidRefreshToken) {
		t.Errorf("expected ErrInvalidRefreshToken, got %v", err)
	}

	// 9. Logout
	if err = srv.Logout(ctx, newRefreshToken); err != nil {
		t.Fatalf("Logout failed: %v", err)
	}

	// 10. Refresh after logout
	if _, _, err = srv.Refresh(ctx, newRefreshToken, "", ""); !errors.Is(err, errs.ErrInvalidRefreshToken) {
		t.Errorf("expected ErrInvalidRefreshToken after logout, got %v", err)
	}

	// 11. Expired token
	expiredTokenValue := "expired-token"
	expiredToken := &domainToken.RefreshToken{
		SessionID: loginSessionID,
		UserID:    u.ID,
		TokenHash: domainToken.HashRefreshToken(expiredTokenValue),
		ExpiresAt: time.Now().Add(-1 * time.Hour),
		Status:    domainToken.RefreshTokenStatusActive,
	}
	_ = rtRepo.Create(ctx, expiredToken)
	if _, _, err = srv.Refresh(ctx, expiredTokenValue, "", ""); !errors.Is(err, errs.ErrInvalidRefreshToken) {
		t.Errorf("expected ErrInvalidRefreshToken for expired token, got %v", err)
	}
}

func TestRevokeSessionPreventsRefresh(t *testing.T) {
	userRepo := newMockUserRepo()
	rtRepo := newMockRefreshTokenRepo()
	sessionRepo := newMockUserSessionRepo()
	srv := newTestService(userRepo, rtRepo, sessionRepo, nil)
	ctx := context.Background()

	u := registerAndVerify(t, srv, userRepo, "sessions@example.com", "password123")

	_, refreshToken, err := srv.Login(ctx, auth2.LoginRequest{Email: "sessions@example.com", Password: "password123", DeviceName: "Test Device"})
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

	if _, _, err = srv.Refresh(ctx, refreshToken, "", ""); !errors.Is(err, errs.ErrInvalidRefreshToken) {
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
	srv := newTestService(userRepo, newMockRefreshTokenRepo(), newMockUserSessionRepo(), nil)
	ctx := context.Background()

	if err := srv.Register(ctx, auth2.RegisterRequest{Email: "newuser@example.com", Password: "pass"}); err != nil {
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
	rtRepo := newMockRefreshTokenRepo()
	sessionRepo := newMockUserSessionRepo()
	srv := newTestService(userRepo, rtRepo, sessionRepo, nil)
	ctx := context.Background()

	u := registerAndVerify(t, srv, userRepo, "disable@example.com", "password123")

	_, refreshToken, err := srv.Login(ctx, auth2.LoginRequest{Email: "disable@example.com", Password: "password123"})
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	sessions, _ := srv.ListSessions(ctx, u.ID)
	if len(sessions) != 1 {
		t.Fatalf("expected one active session, got %d", len(sessions))
	}

	if err := srv.DeactivateUser(ctx, u.ID); err != nil {
		t.Fatalf("DeactivateUser failed: %v", err)
	}

	activeSessions, _ := srv.ListSessions(ctx, u.ID)
	if len(activeSessions) != 0 {
		t.Fatalf("expected no active sessions after deactivate, got %d", len(activeSessions))
	}

	if _, _, err = srv.Login(ctx, auth2.LoginRequest{Email: "disable@example.com", Password: "password123"}); !errors.Is(err, errs.ErrUserDeactivated) {
		t.Errorf("expected ErrUserDeactivated on login after deactivation, got %v", err)
	}

	_, _, err = srv.Refresh(ctx, refreshToken, "", "")
	if !errors.Is(err, errs.ErrInvalidRefreshToken) && !errors.Is(err, errs.ErrUserDeactivated) {
		t.Errorf("expected ErrInvalidRefreshToken or ErrUserDeactivated after deactivation, got %v", err)
	}
}

func TestReactivateUserAllowsLogin(t *testing.T) {
	userRepo := newMockUserRepo()
	srv := newTestService(userRepo, newMockRefreshTokenRepo(), newMockUserSessionRepo(), nil)
	ctx := context.Background()

	u := registerAndVerify(t, srv, userRepo, "reactivate@example.com", "password123")

	if err := srv.DeactivateUser(ctx, u.ID); err != nil {
		t.Fatalf("DeactivateUser failed: %v", err)
	}
	if _, _, err := srv.Login(ctx, auth2.LoginRequest{Email: "reactivate@example.com", Password: "password123"}); !errors.Is(err, errs.ErrUserDeactivated) {
		t.Fatalf("expected ErrUserDeactivated after deactivate, got %v", err)
	}

	if err := srv.ReactivateUser(ctx, u.ID); err != nil {
		t.Fatalf("ReactivateUser failed: %v", err)
	}

	if _, _, err := srv.Login(ctx, auth2.LoginRequest{Email: "reactivate@example.com", Password: "password123"}); err != nil {
		t.Fatalf("expected login to succeed after reactivate, got %v", err)
	}
}

func TestLoginStoresRefreshTokenHashOnly(t *testing.T) {
	userRepo := newMockUserRepo()
	rtRepo := newMockRefreshTokenRepo()
	srv := newTestService(userRepo, rtRepo, newMockUserSessionRepo(), nil)
	ctx := context.Background()

	u := registerAndVerify(t, srv, userRepo, "security@example.com", "password123")
	_ = u

	_, refreshToken, err := srv.Login(ctx, auth2.LoginRequest{Email: "security@example.com", Password: "password123"})
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	hash := domainToken.HashRefreshToken(refreshToken)
	if _, ok := rtRepo.tokens[hash]; !ok {
		t.Fatal("expected token hash stored in repository")
	}
	if _, ok := rtRepo.tokens[refreshToken]; ok {
		t.Fatal("plain refresh token must not be stored as key")
	}
}

func TestLoginNewDeviceCreatesDeviceRecord(t *testing.T) {
	userRepo := newMockUserRepo()
	rtRepo := newMockRefreshTokenRepo()
	sessionRepo := newMockUserSessionRepo()
	deviceRepo := newMockUserDeviceRepo()
	publisher := &mockEventPublisher{}

	srv := auth2.NewAuthService(
		userRepo, rtRepo, sessionRepo, deviceRepo,
		newMockRoleRepo(), &mockTokenManager{}, &mockMailer{}, publisher,
		720*time.Hour, "http://localhost:3222", "App", nil, nil,
	)
	ctx := context.Background()

	u := registerAndVerify(t, srv, userRepo, "notify@example.com", "password123")
	_ = u

	_, _, err := srv.Login(ctx, auth2.LoginRequest{
		Email:      "notify@example.com",
		Password:   "password123",
		DeviceName: "MyPhone",
		UserAgent:  "UA/1.0",
		IPAddress:  "1.2.3.4",
	})
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	if deviceRepo.created == nil {
		t.Fatal("expected deviceRepo.Create to be called for new device")
	}

	// New-device email is published via outbox
	deadline := time.Now().Add(3 * time.Second)
	var found *domainevent.Event
	for time.Now().Before(deadline) {
		for i := range publisher.events {
			if publisher.events[i].Name == domainevent.EmailSend {
				found = &publisher.events[i]
				break
			}
		}
		if found != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if found == nil {
		t.Fatalf("expected an outbox email.send event, got events: %v", publisher.events)
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
