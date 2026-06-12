package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	domain "github.com/unowned-22/api/internal/domain/user"
	"github.com/unowned-22/api/internal/errs"
)

// ── mock: UserRepository ─────────────────────────────────────────────────────

type mockRepo struct {
	users map[string]*domain.User
	idMap map[int64]*domain.User
	seq   int64
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		users: make(map[string]*domain.User),
		idMap: make(map[int64]*domain.User),
	}
}

func (m *mockRepo) Create(ctx context.Context, u *domain.User) error {
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

func (m *mockRepo) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	u, ok := m.users[email]
	if !ok {
		return nil, errs.ErrUserNotFound
	}
	return u, nil
}

func (m *mockRepo) GetByID(ctx context.Context, id int64) (*domain.User, error) {
	u, ok := m.idMap[id]
	if !ok {
		return nil, errs.ErrUserNotFound
	}
	return u, nil
}

// ── mock: RefreshTokenRepository ─────────────────────────────────────────────

type mockRefreshTokenRepo struct {
	tokens map[string]*domain.RefreshToken
	seq    int64
}

func newMockRefreshTokenRepo() *mockRefreshTokenRepo {
	return &mockRefreshTokenRepo{tokens: make(map[string]*domain.RefreshToken)}
}

func (m *mockRefreshTokenRepo) Create(ctx context.Context, t *domain.RefreshToken) error {
	m.seq++
	t.ID = m.seq
	cp := *t
	m.tokens[t.Token] = &cp
	return nil
}

func (m *mockRefreshTokenRepo) GetByToken(ctx context.Context, token string) (*domain.RefreshToken, error) {
	t, ok := m.tokens[token]
	if !ok {
		return nil, errs.ErrRefreshTokenNotFound
	}
	return t, nil
}

func (m *mockRefreshTokenRepo) Revoke(ctx context.Context, token string) error {
	t, ok := m.tokens[token]
	if !ok {
		return errs.ErrRefreshTokenNotFound
	}
	t.Revoked = true
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

// ── mock: RoleRepository ─────────────────────────────────────────────────────

type mockRoleRepo struct {
	byName map[string]*domain.Role
	byID   map[int64]*domain.Role
}

func newMockRoleRepo() *mockRoleRepo {
	userRole := &domain.Role{ID: 2, Name: "user"}
	adminRole := &domain.Role{ID: 1, Name: "admin"}
	return &mockRoleRepo{
		byName: map[string]*domain.Role{"user": userRole, "admin": adminRole},
		byID:   map[int64]*domain.Role{2: userRole, 1: adminRole},
	}
}

func (m *mockRoleRepo) GetByID(ctx context.Context, id int64) (*domain.Role, error) {
	r, ok := m.byID[id]
	if !ok {
		return nil, errs.ErrRoleNotFound
	}
	return r, nil
}

func (m *mockRoleRepo) GetByName(ctx context.Context, name string) (*domain.Role, error) {
	r, ok := m.byName[name]
	if !ok {
		return nil, errs.ErrRoleNotFound
	}
	return r, nil
}

func (m *mockRoleRepo) List(ctx context.Context) ([]*domain.Role, error) {
	roles := make([]*domain.Role, 0, len(m.byName))
	for _, r := range m.byName {
		roles = append(roles, r)
	}
	return roles, nil
}

// ── mock: TokenManagerExtended ───────────────────────────────────────────────

type mockTokenManager struct{}

func (m *mockTokenManager) Generate(userID int64) (string, error) {
	return "mock-token-for-user", nil
}

func (m *mockTokenManager) Parse(token string) (int64, error) {
	if token == "mock-token-for-user" {
		return 1, nil
	}
	return 0, errors.New("invalid token")
}

func (m *mockTokenManager) GenerateWithRole(userID int64, role string) (string, error) {
	return "mock-token-for-user", nil
}

func (m *mockTokenManager) ParseWithRole(token string) (int64, string, error) {
	if token == "mock-token-for-user" {
		return 1, "user", nil
	}
	return 0, "", errors.New("invalid token")
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestAuthService(t *testing.T) {
	repo := newMockRepo()
	refreshTokenRepo := newMockRefreshTokenRepo()
	roleRepo := newMockRoleRepo()
	tm := &mockTokenManager{}
	srv := NewAuthService(repo, refreshTokenRepo, roleRepo, tm)

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

	// 6. Refresh success
	newAccessToken, err := srv.Refresh(ctx, refreshToken)
	if err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}
	if newAccessToken != "mock-token-for-user" {
		t.Errorf("unexpected refreshed access token: %s", newAccessToken)
	}

	// 7. Refresh with unknown token
	_, err = srv.Refresh(ctx, "non-existent-token")
	if !errors.Is(err, errs.ErrInvalidRefreshToken) {
		t.Errorf("expected ErrInvalidRefreshToken, got %v", err)
	}

	// 8. Logout
	if err = srv.Logout(ctx, refreshToken); err != nil {
		t.Fatalf("Logout failed: %v", err)
	}

	// 9. Refresh after logout (revoked)
	_, err = srv.Refresh(ctx, refreshToken)
	if !errors.Is(err, errs.ErrInvalidRefreshToken) {
		t.Errorf("expected ErrInvalidRefreshToken after logout, got %v", err)
	}

	// 10. Refresh expired token
	expiredToken := &domain.RefreshToken{
		UserID:    1,
		Token:     "expired-token",
		ExpiresAt: time.Now().Add(-1 * time.Hour),
		Revoked:   false,
	}
	_ = refreshTokenRepo.Create(ctx, expiredToken)
	_, err = srv.Refresh(ctx, "expired-token")
	if !errors.Is(err, errs.ErrInvalidRefreshToken) {
		t.Errorf("expected ErrInvalidRefreshToken for expired token, got %v", err)
	}
}

func TestRegisterAssignsDefaultRole(t *testing.T) {
	repo := newMockRepo()
	srv := NewAuthService(repo, newMockRefreshTokenRepo(), newMockRoleRepo(), &mockTokenManager{})
	ctx := context.Background()

	if err := srv.Register(ctx, RegisterRequest{Email: "newuser@example.com", Password: "pass"}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	user, err := repo.GetByEmail(ctx, "newuser@example.com")
	if err != nil {
		t.Fatalf("GetByEmail failed: %v", err)
	}
	if user.RoleID != 2 {
		t.Errorf("expected RoleID=2 (user), got %d", user.RoleID)
	}
	if user.RoleName != "user" {
		t.Errorf("expected RoleName='user', got '%s'", user.RoleName)
	}
}
