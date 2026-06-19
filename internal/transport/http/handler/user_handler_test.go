package handler

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/unowned-22/api/internal/domain/user"
)

// minimal mock service implementing ListUsers
type mockUserService struct {
	users []*user.User
	total int64
	err   error
}

func (m *mockUserService) ListUsers(_ context.Context, page, limit int) ([]*user.User, int64, error) {
	return m.users, m.total, m.err
}

func (m *mockUserService) GetProfile(_ context.Context, userID int64) (*user.User, error) {
	if len(m.users) == 0 {
		return nil, nil
	}
	return m.users[0], nil
}

func (m *mockUserService) UpdateProfile(_ context.Context, userID int64, fullName, username, phone string) error {
	if len(m.users) == 0 {
		return nil
	}
	m.users[0].FullName = fullName
	m.users[0].Username = username
	m.users[0].Phone = phone
	return nil
}

func (m *mockUserService) UploadAvatar(ctx context.Context, userID int64, file io.Reader, size int64, contentType string) (string, error) {
	return "", nil
}

func (m *mockUserService) UploadCover(ctx context.Context, userID int64, file io.Reader, size int64, contentType string) (string, error) {
	return "", nil
}

func (m *mockUserService) DeleteAvatar(ctx context.Context, userID int64) error {
	return nil
}

func (m *mockUserService) DeleteCover(ctx context.Context, userID int64) error {
	return nil
}

func TestUserHandler_List(t *testing.T) {
	// prepare handler with mock service
	svc := &mockUserService{
		users: []*user.User{{ID: 1, Email: "a@x.com", RoleName: "user", CreatedAt: time.Now()}},
		total: 1,
	}
	h := &UserHandler{userService: svc}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users?page=1&limit=10", nil)
	w := httptest.NewRecorder()

	h.List(w, req)

	res := w.Result()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
	// simple body check
	b, _ := io.ReadAll(res.Body)
	if !strings.Contains(string(b), "data") {
		t.Fatalf("expected response body to include data, got: %s", string(b))
	}
}
