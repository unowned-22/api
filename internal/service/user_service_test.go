package service

import (
	"context"
	"errors"
	"testing"

	"github.com/unowned-22/api/internal/domain/user"
	"github.com/unowned-22/api/internal/errs"
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

// ── tests ─────────────────────────────────────────────────────────────────────

func TestUserService_GetProfile(t *testing.T) {
	repo := newMockUserRepo()
	srv := NewUserService(repo)

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
