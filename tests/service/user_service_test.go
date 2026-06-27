package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/unowned-22/api/internal/domain/user"
	"github.com/unowned-22/api/internal/errs"
	service2 "github.com/unowned-22/api/internal/service"
)

// ── mock: UserRepository ─────────────────────────────────────────────────────

type mockUserRepo struct {
	users map[string]*user.User
	idMap map[int64]*user.User
	seq   int64
}

func newMockUserRepo() *mockUserRepo {
	return &mockUserRepo{
		users: make(map[string]*user.User),
		idMap: make(map[int64]*user.User),
	}
}

func (m *mockUserRepo) Create(ctx context.Context, u *user.User) error {
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

func (m *mockUserRepo) GetByEmail(ctx context.Context, email string) (*user.User, error) {
	u, ok := m.users[email]
	if !ok {
		return nil, errs.ErrUserNotFound
	}
	return u, nil
}

func (m *mockUserRepo) GetByID(ctx context.Context, id int64) (*user.User, error) {
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

func (m *mockUserRepo) GetByVerificationToken(ctx context.Context, token string) (*user.User, error) {
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

func (m *mockUserRepo) List(ctx context.Context, offset int, limit int) ([]*user.User, error) {
	var out []*user.User
	for _, u := range m.idMap {
		out = append(out, u)
	}
	if offset >= len(out) {
		return []*user.User{}, nil
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

func (m *mockUserRepo) IncrementTokenVersion(ctx context.Context, userID int64) error {
	u, ok := m.idMap[userID]
	if !ok {
		return errs.ErrUserNotFound
	}
	u.TokenVersion++
	return nil
}

func (m *mockUserRepo) GetByUsername(ctx context.Context, username string) (*user.User, error) {
	for _, u := range m.idMap {
		if u.Username == username {
			return u, nil
		}
	}
	return nil, errs.ErrUserNotFound
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestUserService_GetProfile(t *testing.T) {
	repo := newMockUserRepo()
	srv := service2.NewUserService(repo, nil, nil, "app-uploads")

	ctx := context.Background()

	// Seed user
	u := &user.User{
		Email:    "test@example.com",
		Password: "password123",
		RoleID:   2,
	}
	if err := repo.Create(ctx, u); err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	// Test Profile Success
	fetched, err := srv.GetProfile(ctx, u.ID)
	if err != nil {
		t.Fatalf("GetProfile failed: %v", err)
	}
	if fetched.Email != "test@example.com" {
		t.Errorf("unexpected email: %s", fetched.Email)
	}
	if fetched.RoleName != "user" {
		t.Errorf("expected role 'user', got '%s'", fetched.RoleName)
	}

	// Test Profile NotFound
	_, err = srv.GetProfile(ctx, 999)
	if !errors.Is(err, errs.ErrUserNotFound) {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}
