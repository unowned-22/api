package closefriend

import "context"

type CloseFriend struct {
	OwnerID  int64 `json:"owner_id"`
	FriendID int64 `json:"friend_id"`
}

type Repository interface {
	Add(ctx context.Context, ownerID, friendID int64) error
	Remove(ctx context.Context, ownerID, friendID int64) error
	List(ctx context.Context, ownerID int64) ([]int64, error)
	IsCloseFriend(ctx context.Context, ownerID, friendID int64) (bool, error)
}

type Service interface {
	Add(ctx context.Context, ownerID, friendID int64) error
	Remove(ctx context.Context, ownerID, friendID int64) error
	List(ctx context.Context, ownerID int64) ([]int64, error)
	IsCloseFriend(ctx context.Context, ownerID, friendID int64) (bool, error)
}
