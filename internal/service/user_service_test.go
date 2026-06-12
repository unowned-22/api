package service

import (
	"context"
	"errors"
	"testing"

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

// ── tests ─────────────────────────────────────────────────────────────────────

func TestUserService_GetProfile(t *testing.T) {
	repo := newMockRepo()
	srv := NewUserService(repo)

	ctx := context.Background()

	// Seed user
	user := &domain.User{
		Email:    "test@example.com",
		Password: "password123",
		RoleID:   2,
	}
	if err := repo.Create(ctx, user); err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	// Test Profile Success
	fetchedUser, err := srv.GetProfile(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetProfile failed: %v", err)
	}
	if fetchedUser.Email != "test@example.com" {
		t.Errorf("unexpected email: %s", fetchedUser.Email)
	}
	if fetchedUser.RoleName != "user" {
		t.Errorf("expected role 'user', got '%s'", fetchedUser.RoleName)
	}

	// Test Profile NotFound
	_, err = srv.GetProfile(ctx, 999)
	if !errors.Is(err, errs.ErrUserNotFound) {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}
