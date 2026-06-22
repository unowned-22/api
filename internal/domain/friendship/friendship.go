package friendship

import (
	"context"
	"time"

	"github.com/unowned-22/api/internal/pagination"
)

type Status string

const (
	StatusPending  Status = "pending"
	StatusAccepted Status = "accepted"
	StatusRejected Status = "rejected"
)

// Friendship — directed request/relation from RequesterID to AddresseeID.
type Friendship struct {
	ID          int64     `json:"id"`
	RequesterID int64     `json:"requester_id"`
	AddresseeID int64     `json:"addressee_id"`
	Status      Status    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Repository interface {
	Create(ctx context.Context, requesterID, addresseeID int64) (*Friendship, error)
	UpdateStatus(ctx context.Context, id int64, status Status) (*Friendship, error)
	GetByUsers(ctx context.Context, userA, userB int64) (*Friendship, error)
	GetByID(ctx context.Context, id int64) (*Friendship, error)
	Delete(ctx context.Context, id int64) error

	ListFriends(ctx context.Context, userID int64, page pagination.Query) ([]*Friendship, int64, error)
	ListIncomingRequests(ctx context.Context, userID int64, page pagination.Query) ([]*Friendship, int64, error)
	ListOutgoingRequests(ctx context.Context, userID int64, page pagination.Query) ([]*Friendship, int64, error)

	IsFriend(ctx context.Context, userA, userB int64) (bool, error)
	IsSubscriber(ctx context.Context, requesterID, addresseeID int64) (bool, error)
	GetFriendIDs(ctx context.Context, userID int64) ([]int64, error)
}

type Service interface {
	SendRequest(ctx context.Context, requesterID, addresseeID int64) (*Friendship, error)
	Accept(ctx context.Context, userID int64, friendshipID int64) (*Friendship, error)
	Reject(ctx context.Context, userID int64, friendshipID int64) (*Friendship, error)
	Cancel(ctx context.Context, userID int64, friendshipID int64) error
	Remove(ctx context.Context, userID int64, friendshipID int64) error

	ListFriends(ctx context.Context, userID int64, page pagination.Query) ([]*Friendship, int64, error)
	ListIncomingRequests(ctx context.Context, userID int64, page pagination.Query) ([]*Friendship, int64, error)
	ListOutgoingRequests(ctx context.Context, userID int64, page pagination.Query) ([]*Friendship, int64, error)
	IsFriend(ctx context.Context, userA, userB int64) (bool, error)
}
