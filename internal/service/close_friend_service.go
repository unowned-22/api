package service

import (
	"context"

	"github.com/unowned-22/api/internal/domain/closefriend"
	"github.com/unowned-22/api/internal/domain/friendship"
	"github.com/unowned-22/api/internal/errs"
)

type closeFriendService struct {
	repo         closefriend.Repository
	friendshipRs friendship.Repository
}

func NewCloseFriendService(repo closefriend.Repository, friendshipRepo friendship.Repository) closefriend.Service {
	return &closeFriendService{repo: repo, friendshipRs: friendshipRepo}
}

func (s *closeFriendService) Add(ctx context.Context, ownerID, friendID int64) error {
	if ownerID == friendID {
		return errs.ErrCannotFriendYourself
	}
	isFriend, err := s.friendshipRs.IsFriend(ctx, ownerID, friendID)
	if err != nil {
		return err
	}
	if !isFriend {
		return errs.ErrNotFriend
	}
	return s.repo.Add(ctx, ownerID, friendID)
}

func (s *closeFriendService) Remove(ctx context.Context, ownerID, friendID int64) error {
	ok, err := s.repo.IsCloseFriend(ctx, ownerID, friendID)
	if err != nil {
		return err
	}
	if !ok {
		return errs.ErrCloseFriendNotFound
	}
	return s.repo.Remove(ctx, ownerID, friendID)
}

func (s *closeFriendService) List(ctx context.Context, ownerID int64) ([]int64, error) {
	return s.repo.List(ctx, ownerID)
}

func (s *closeFriendService) IsCloseFriend(ctx context.Context, ownerID, friendID int64) (bool, error) {
	return s.repo.IsCloseFriend(ctx, ownerID, friendID)
}

var _ closefriend.Service = (*closeFriendService)(nil)
