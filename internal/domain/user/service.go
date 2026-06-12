package user

import "context"

type UserService interface {
	GetProfile(ctx context.Context, userID int64) (*User, error)
}
