package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	domainevent "github.com/unowned-22/api/internal/domain/event"
	domainmailer "github.com/unowned-22/api/internal/domain/mailer"
	domainRole "github.com/unowned-22/api/internal/domain/role"
	domainToken "github.com/unowned-22/api/internal/domain/token"
	domainUser "github.com/unowned-22/api/internal/domain/user"
	"github.com/unowned-22/api/internal/errs"
)

// ── mock: EventPublisher ─────────────────────────────────────────────────────

type mockEventPublisher struct{}

func (m *mockEventPublisher) Publish(ctx context.Context, event domainevent.Event) error {
	return nil
}

func (m *mockEventPublisher) Close() error {
	return nil
}

type mockMailer struct {
	sendErr error
}

func (m *mockMailer) Send(ctx context.Context, msg domainmailer.Message) error {
	return m.sendErr
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
	m.tokens[t.Token] = &cp
	return nil
}

func (m *mockRefreshTokenRepo) GetByToken(ctx context.Context, tokenStr string) (*domainToken.RefreshToken, error) {
	t, ok := m.tokens[tokenStr]
	if !ok {
		return nil, errs.ErrRefreshTokenNotFound
	}
	return t, nil
}

func (m *mockRefreshTokenRepo) RevokeRefreshToken(ctx context.Context, tokenStr string) error {
	t, ok := m.tokens[tokenStr]
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

func (m *mockTokenManager) GenerateWithRole(userID int64, role string) (string, error) {
	return "mock-token-for-user", nil
}

func (m *mockTokenManager) ParseWithRole(tokenStr string) (int64, string, error) {
	if tokenStr == "mock-token-for-user" {
		return 1, "user", nil
	}
	return 0, "", errors.New("invalid token")
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestAuthService(t *testing.T) {
	userRepo := newMockUserRepo()
	refreshTokenRepo := newMockRefreshTokenRepo()
	roleRepo := newMockRoleRepo()
	tm := &mockTokenManager{}
	mailer := &mockMailer{}
	publisher := &mockEventPublisher{}
	srv := NewAuthService(userRepo, refreshTokenRepo, roleRepo, tm, mailer, publisher, "http://localhost:3222", "App")

	ctx := context.Background()

	// 1. Register
	err := srv.Register(ctx, RegisterRequest{Email: "test@example.com", Password: "password123"})
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

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
	newAccessToken, newRefreshToken, err := srv.Refresh(ctx, refreshToken)
	if err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}
	if newAccessToken != "mock-token-for-user" {
		t.Errorf("unexpected refreshed access token: %s", newAccessToken)
	}
	if newRefreshToken == "" || newRefreshToken == refreshToken {
		t.Errorf("expected a new refresh token")
	}

	// 7. Reusing the old refresh token must fail.
	_, _, err = srv.Refresh(ctx, refreshToken)
	if !errors.Is(err, errs.ErrInvalidRefreshToken) {
		t.Errorf("expected ErrInvalidRefreshToken for reused token, got %v", err)
	}

	// 8. Refresh with unknown token
	_, _, err = srv.Refresh(ctx, "non-existent-token")
	if !errors.Is(err, errs.ErrInvalidRefreshToken) {
		t.Errorf("expected ErrInvalidRefreshToken, got %v", err)
	}

	// 9. Logout revokes the active refresh token.
	if err = srv.Logout(ctx, newRefreshToken); err != nil {
		t.Fatalf("Logout failed: %v", err)
	}

	// 10. Refresh after logout (revoked)
	_, _, err = srv.Refresh(ctx, newRefreshToken)
	if !errors.Is(err, errs.ErrInvalidRefreshToken) {
		t.Errorf("expected ErrInvalidRefreshToken after logout, got %v", err)
	}

	// 11. Refresh expired token
	expiredToken := &domainToken.RefreshToken{
		UserID:    1,
		Token:     "expired-token",
		ExpiresAt: time.Now().Add(-1 * time.Hour),
		Status:    domainToken.RefreshTokenStatusActive,
	}
	_ = refreshTokenRepo.CreateRefreshToken(ctx, expiredToken)
	_, _, err = srv.Refresh(ctx, "expired-token")
	if !errors.Is(err, errs.ErrInvalidRefreshToken) {
		t.Errorf("expected ErrInvalidRefreshToken for expired token, got %v", err)
	}
}

func TestRegisterAssignsDefaultRole(t *testing.T) {
	userRepo := newMockUserRepo()
	mailer := &mockMailer{}
	publisher := &mockEventPublisher{}
	srv := NewAuthService(userRepo, newMockRefreshTokenRepo(), newMockRoleRepo(), &mockTokenManager{}, mailer, publisher, "http://localhost:3222", "App")
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
