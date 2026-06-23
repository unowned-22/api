package service

import (
	"context"
	"fmt"

	"github.com/unowned-22/api/internal/domain/friendship"
	"github.com/unowned-22/api/internal/domain/profile"
	"github.com/unowned-22/api/internal/domain/user"
	"github.com/unowned-22/api/internal/domain/userprivacy"
	"github.com/unowned-22/api/internal/errs"
)

type ProfileService struct {
	userRepo       user.UserRepository
	friendshipSvc  friendship.Service
	friendshipRepo friendship.Repository
	privacyRepo    userprivacy.Repository
}

func NewProfileService(u user.UserRepository, fRepo friendship.Repository, pRepo userprivacy.Repository, fSvc friendship.Service) *ProfileService {
	return &ProfileService{userRepo: u, friendshipRepo: fRepo, privacyRepo: pRepo, friendshipSvc: fSvc}
}

func (s *ProfileService) GetPublicProfile(ctx context.Context, viewerID int64, username string) (*profile.PublicProfile, error) {
	tgt, err := s.userRepo.GetByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	if tgt.DeactivatedAt != nil {
		return nil, errs.ErrUserNotFound
	}

	var rel profile.Relation
	if viewerID == tgt.ID {
		rel = profile.RelationSelf
	} else {
		f, ferr := s.friendshipRepo.GetByUsers(ctx, viewerID, tgt.ID)
		if ferr != nil {
			return nil, fmt.Errorf("failed to get friendship: %w", ferr)
		}
		if f == nil {
			rel = profile.RelationNone
		} else if f.Status == friendship.StatusAccepted {
			rel = profile.RelationFriends
		} else if f.Status == friendship.StatusPending && f.RequesterID == viewerID {
			rel = profile.RelationOutgoingRequest
		} else if f.Status == friendship.StatusPending && f.AddresseeID == viewerID {
			rel = profile.RelationIncomingRequest
		} else {
			rel = profile.RelationNone
		}
	}

	priv, perr := s.privacyRepo.GetByUserID(ctx, tgt.ID)
	if perr != nil {
		return nil, fmt.Errorf("failed to get privacy settings: %w", perr)
	}

	p := &profile.PublicProfile{
		ID:        tgt.ID,
		Username:  tgt.Username,
		FullName:  tgt.FullName,
		AvatarURL: tgt.AvatarURL,
		CoverURL:  tgt.CoverURL,
		Relation:  rel,
		CreatedAt: tgt.CreatedAt,
	}

	isSelf := rel == profile.RelationSelf
	isFriend := rel == profile.RelationFriends

	// Email
	if isSelf || priv.ShowEmail == userprivacy.VisibilityEveryone || (priv.ShowEmail == userprivacy.VisibilityFriends && isFriend) {
		// hide email if empty
		if tgt.Email != "" {
			e := tgt.Email
			p.Email = &e
		}
	}

	// Phone
	if isSelf || priv.ShowPhone == userprivacy.VisibilityEveryone || (priv.ShowPhone == userprivacy.VisibilityFriends && isFriend) {
		if tgt.Phone != "" {
			ph := tgt.Phone
			p.Phone = &ph
		}
	}

	// Friends count
	if isSelf || priv.ShowFriends == userprivacy.VisibilityEveryone || (priv.ShowFriends == userprivacy.VisibilityFriends && isFriend) {
		cnt, cerr := s.friendshipRepo.CountFriends(ctx, tgt.ID)
		if cerr != nil {
			return nil, fmt.Errorf("failed to count friends: %w", cerr)
		}
		pcount := cnt
		p.FriendsCount = &pcount
	}

	return p, nil
}

// compile-time check
var _ profile.Service = (*ProfileService)(nil)
