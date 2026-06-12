package user

import "context"

type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type UserService interface {
	Register(ctx context.Context, req RegisterRequest) error
	Login(ctx context.Context, req LoginRequest) (string, string, error)
	Refresh(ctx context.Context, refreshToken string) (string, error)
	Logout(ctx context.Context, refreshToken string) error
	Profile(ctx context.Context, id int64) (*User, error)
}
