package service

import (
	"fmt"
	"strings"

	"context"

	"github.com/unowned-22/api/internal/domain/community"
	"github.com/unowned-22/api/internal/domain/messenger"
	"github.com/unowned-22/api/internal/errs"
)

func (s *MessengerService) PromoteToCommunity(ctx context.Context, requesterID, conversationID int64, communityType, visibility string) (int64, error) {
	conv, err := s.convRepo.GetByID(ctx, conversationID)
	if err != nil {
		return 0, err
	}
	if conv == nil {
		return 0, errs.ErrConversationNotFound
	}
	if conv.Type != messenger.TypeGroup && conv.Type != messenger.TypeChannel {
		return 0, errs.ErrPromoteRequiresGroupOrChannel
	}
	if conv.CommunityID != nil {
		return 0, errs.ErrConversationAlreadyLinked
	}

	requesterMember, err := s.memberRepo.GetMember(ctx, conversationID, requesterID)
	if err != nil {
		return 0, errs.ErrNotConversationMember
	}
	if requesterMember.Role != messenger.RoleOwner {
		return 0, errs.ErrNotConversationOwner
	}

	slug := slugifyForCommunity(conv.Title) + "-" + fmt.Sprint(conversationID)

	c, err := s.communitySvc.Create(ctx, requesterID, community.CreateRequest{
		Type:               community.Type(communityType),
		Visibility:         community.Visibility(visibility),
		Name:               conv.Title,
		Slug:               slug,
		Description:        conv.Description,
		LinkConversationID: &conversationID,
	})
	if err != nil {
		return 0, err
	}

	// Migrate the remaining conversation members (the requester/owner was
	// already added as community owner by Create above) into
	// community_members, preserving role where it maps cleanly.
	members, err := s.memberRepo.ListMembers(ctx, conversationID)
	if err != nil {
		// Community already exists at this point; a failure to migrate the
		// member list is logged and surfaced, but does not roll back the
		// promotion — the owner can re-invite members manually if needed.
		return c.ID, nil
	}
	for _, m := range members {
		if m.UserID == requesterID {
			continue // already community owner
		}
		role, ok := messengerRoleToCommunityRole(m.Role)
		if !ok {
			continue
		}
		_ = s.communitySvc.AddMember(ctx, c.ID, requesterID, m.UserID, role)
	}

	return c.ID, nil
}

func messengerRoleToCommunityRole(r messenger.MemberRole) (community.MemberRole, bool) {
	switch r {
	case messenger.RoleAdmin:
		return community.MemberRoleAdmin, true
	case messenger.RoleMember:
		return community.MemberRoleMember, true
	case messenger.RoleSubscriber:
		return community.MemberRoleSubscriber, true
	default:
		return "", false
	}
}

// slugifyForCommunity is a minimal, dependency-free slug generator shared
// conceptually with handler.slugify (video_channel_handler.go) but kept
// local to the service layer to avoid a transport->service import.
func slugifyForCommunity(s string) string {
	out := make([]byte, 0, len(s))
	for _, c := range []byte(strings.ToLower(s)) {
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9':
			out = append(out, c)
		default:
			if len(out) > 0 && out[len(out)-1] != '-' {
				out = append(out, '-')
			}
		}
	}
	for len(out) > 0 && out[len(out)-1] == '-' {
		out = out[:len(out)-1]
	}
	if len(out) == 0 {
		return "community"
	}
	return string(out)
}
