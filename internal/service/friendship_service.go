package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/unowned-22/api/internal/domain/event"
	"github.com/unowned-22/api/internal/domain/friendship"
	"github.com/unowned-22/api/internal/errs"
	"github.com/unowned-22/api/internal/pagination"
)

type FriendshipService struct {
	repo      friendship.Repository
	publisher event.Publisher
}

func NewFriendshipService(repo friendship.Repository, publisher event.Publisher) *FriendshipService {
	return &FriendshipService{repo: repo, publisher: publisher}
}

func (s *FriendshipService) SendRequest(ctx context.Context, requesterID, addresseeID int64) (*friendship.Friendship, error) {
	if requesterID == addresseeID {
		return nil, errs.ErrCannotFriendYourself
	}

	existing, err := s.repo.GetByUsers(ctx, requesterID, addresseeID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if existing.Status == friendship.StatusPending || existing.Status == friendship.StatusAccepted {
			return nil, errs.ErrFriendshipAlreadyExist
		}
		// if rejected: if same direction, reopen; otherwise insert new directed row
		if existing.RequesterID == requesterID && existing.AddresseeID == addresseeID {
			f, err := s.repo.UpdateStatus(ctx, existing.ID, friendship.StatusPending)
			if err != nil {
				return nil, err
			}
			// publish event
			payload, _ := json.Marshal(map[string]interface{}{"friendship_id": f.ID, "requester_id": f.RequesterID, "addressee_id": f.AddresseeID})
			_ = s.publisher.Publish(ctx, event.Event{Name: event.FriendRequestReceived, Payload: payload})
			return f, nil
		}
	}

	f, err := s.repo.Create(ctx, requesterID, addresseeID)
	if err != nil {
		return nil, err
	}
	payload, _ := json.Marshal(map[string]interface{}{"friendship_id": f.ID, "requester_id": f.RequesterID, "addressee_id": f.AddresseeID})
	if pubErr := s.publisher.Publish(ctx, event.Event{Name: event.FriendRequestReceived, Payload: payload}); pubErr != nil {
		// best-effort logging; don't fail the operation
		fmt.Printf("failed to publish friend.request_received: %v\n", pubErr)
	}
	return f, nil
}

func (s *FriendshipService) Accept(ctx context.Context, userID int64, friendshipID int64) (*friendship.Friendship, error) {
	f, err := s.repo.GetByID(ctx, friendshipID)
	if err != nil {
		return nil, err
	}
	if f == nil {
		return nil, errs.ErrFriendshipNotFound
	}
	if f.AddresseeID != userID {
		return nil, errs.ErrNotAddressee
	}
	upd, err := s.repo.UpdateStatus(ctx, friendshipID, friendship.StatusAccepted)
	if err != nil {
		return nil, err
	}
	payload, _ := json.Marshal(map[string]interface{}{"friendship_id": f.ID, "requester_id": f.RequesterID, "addressee_id": f.AddresseeID})
	if pubErr := s.publisher.Publish(ctx, event.Event{Name: event.FriendRequestAccepted, Payload: payload}); pubErr != nil {
		fmt.Printf("failed to publish friend.request_accepted: %v\n", pubErr)
	}
	return upd, nil
}

func (s *FriendshipService) Reject(ctx context.Context, userID int64, friendshipID int64) (*friendship.Friendship, error) {
	f, err := s.repo.GetByID(ctx, friendshipID)
	if err != nil {
		return nil, err
	}
	if f == nil {
		return nil, errs.ErrFriendshipNotFound
	}
	if f.AddresseeID != userID {
		return nil, errs.ErrNotAddressee
	}
	updated, err := s.repo.UpdateStatus(ctx, f.ID, friendship.StatusRejected)
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *FriendshipService) Cancel(ctx context.Context, userID int64, friendshipID int64) error {
	f, err := s.repo.GetByID(ctx, friendshipID)
	if err != nil {
		return err
	}
	if f == nil {
		return errs.ErrFriendshipNotFound
	}
	if f.RequesterID != userID {
		return errs.ErrNotRequester
	}
	if f.Status != friendship.StatusPending {
		return fmt.Errorf("cannot cancel non-pending request")
	}
	// delete the pending request to cancel
	return s.repo.Delete(ctx, friendshipID)
}

func (s *FriendshipService) Remove(ctx context.Context, userID int64, friendshipID int64) error {
	f, err := s.repo.GetByID(ctx, friendshipID)
	if err != nil {
		return err
	}
	if f == nil {
		return errs.ErrFriendshipNotFound
	}
	if f.Status != friendship.StatusAccepted {
		return fmt.Errorf("can only remove accepted friendship")
	}
	if f.RequesterID != userID && f.AddresseeID != userID {
		return errs.ErrForbidden
	}
	return s.repo.Delete(ctx, friendshipID)
}

func (s *FriendshipService) ListFriends(ctx context.Context, userID int64, page pagination.Query) ([]*friendship.Friendship, int64, error) {
	return s.repo.ListFriends(ctx, userID, page)
}

func (s *FriendshipService) ListIncomingRequests(ctx context.Context, userID int64, page pagination.Query) ([]*friendship.Friendship, int64, error) {
	return s.repo.ListIncomingRequests(ctx, userID, page)
}

func (s *FriendshipService) ListOutgoingRequests(ctx context.Context, userID int64, page pagination.Query) ([]*friendship.Friendship, int64, error) {
	return s.repo.ListOutgoingRequests(ctx, userID, page)
}

func (s *FriendshipService) IsFriend(ctx context.Context, userA, userB int64) (bool, error) {
	return s.repo.IsFriend(ctx, userA, userB)
}

func (s *FriendshipService) ListSuggestions(ctx context.Context, userID int64, page pagination.Query) ([]*friendship.Suggestion, int64, error) {
	return s.repo.ListSuggestions(ctx, userID, page)
}
