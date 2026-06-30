package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/unowned-22/api/internal/domain/community"
	"github.com/unowned-22/api/internal/domain/messenger"
	"github.com/unowned-22/api/internal/errs"
)

// slugRe allows letters, digits, and hyphens only.
var slugRe = regexp.MustCompile(`^[a-z0-9-]+$`)

// CommunityService implements community.Service.
//
// messengerConvRepo is used only for the Stage 5 auto-chat behaviour:
//   - On Create, a non-general community automatically gets a linked
//     conversation (type=channel for public, type=group for private).
//   - When Create is called with CreateRequest.LinkConversationID set
//     (used internally by MessengerService.PromoteToCommunity), no new
//     conversation is created — the existing one is linked instead.
//
// This is a one-way dependency (community → messenger repo only, never
// messenger.Service), so there is no import cycle: MessengerService is free
// to depend on community.Service for the reverse direction (promote flow).
type CommunityService struct {
	repo              community.Repository
	messengerConvRepo messenger.ConversationRepository
}

func NewCommunityService(repo community.Repository, messengerConvRepo messenger.ConversationRepository) *CommunityService {
	return &CommunityService{repo: repo, messengerConvRepo: messengerConvRepo}
}

// ── creation ─────────────────────────────────────────────────────────────────

func (s *CommunityService) Create(ctx context.Context, ownerID int64, req community.CreateRequest) (*community.Community, error) {
	name := strings.TrimSpace(req.Name)
	slug := strings.TrimSpace(strings.ToLower(req.Slug))
	desc := strings.TrimSpace(req.Description)

	if name == "" || len(name) < 2 || len(name) > 128 {
		return nil, fmt.Errorf("%w: name must be 2–128 characters", errs.ErrInvalidCommunityPayload)
	}
	if slug == "" || len(slug) < 2 || len(slug) > 64 || !slugRe.MatchString(slug) {
		return nil, fmt.Errorf("%w: slug must be 2–64 lowercase letters, digits, or hyphens", errs.ErrInvalidCommunityPayload)
	}
	if req.Type == "" {
		req.Type = community.TypeGeneral
	}
	if req.Visibility == "" {
		req.Visibility = community.VisibilityPublic
	}
	if req.Visibility != community.VisibilityPublic && req.Visibility != community.VisibilityPrivate {
		return nil, fmt.Errorf("%w: visibility must be 'public' or 'private'", errs.ErrInvalidCommunityPayload)
	}

	c := &community.Community{
		OwnerID:     ownerID,
		Type:        req.Type,
		Visibility:  req.Visibility,
		Name:        name,
		Slug:        slug,
		Description: desc,
	}
	if err := s.repo.Create(ctx, c); err != nil {
		return nil, err
	}

	// Auto-enrol the creator as owner member.
	_ = s.repo.AddMember(ctx, &community.Member{
		CommunityID: c.ID,
		UserID:      ownerID,
		Role:        community.MemberRoleOwner,
	})
	// Members count starts at 1 (owner).
	_ = s.repo.IncrMembersCount(ctx, c.ID)
	c.MembersCount = 1

	// ── Stage 5: chat wiring ────────────────────────────────────────────────
	switch {
	case req.LinkConversationID != nil:
		// Called from MessengerService.PromoteToCommunity — the conversation
		// (and its members) already exists; just link it, don't create a
		// second one.
		_ = s.messengerConvRepo.SetCommunityID(ctx, *req.LinkConversationID, &c.ID)

	case c.Type != community.TypeGeneral:
		// Auto-provision a chat for the new community. type=general
		// communities (plain groups/blogs without a dedicated channel
		// purpose) do NOT get one automatically — see AGENTS.md
		// "Communities Feature Guidance".
		convType := messenger.TypeGroup
		if c.Visibility == community.VisibilityPublic {
			convType = messenger.TypeChannel
		}
		now := time.Now().UTC()
		conv := &messenger.Conversation{
			Type:         convType,
			Title:        c.Name,
			Description:  c.Description,
			CreatedBy:    ownerID,
			OwnerID:      &ownerID,
			MembersCount: 1,
			CommunityID:  &c.ID,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		members := []*messenger.ConversationMember{
			{UserID: ownerID, Role: messenger.RoleOwner, JoinedAt: now},
		}
		// Best-effort: a missing auto-chat does not block community creation;
		// the owner can still use the community without messaging.
		_, _ = s.messengerConvRepo.CreateWithMembers(ctx, conv, members)
	}

	return c, nil
}

// AddMember adds targetUserID to communityID with the given role. Only the
// community owner may call this directly (as opposed to Join, which is
// self-service). Used internally by MessengerService.PromoteToCommunity to
// migrate conversation_members → community_members in bulk.
func (s *CommunityService) AddMember(ctx context.Context, communityID, actorID, targetUserID int64, role community.MemberRole) error {
	c, err := s.repo.GetByID(ctx, communityID)
	if err != nil {
		return err
	}
	if c.OwnerID != actorID {
		return errs.ErrCommunityForbidden
	}
	if role == community.MemberRoleOwner {
		return errs.ErrCommunityForbidden // ownership transfer not allowed here
	}
	already, err := s.repo.IsMember(ctx, communityID, targetUserID)
	if err != nil {
		return err
	}
	if already {
		return nil // idempotent — already migrated
	}
	if err := s.repo.AddMember(ctx, &community.Member{
		CommunityID: communityID,
		UserID:      targetUserID,
		Role:        role,
	}); err != nil {
		return err
	}
	return s.repo.IncrMembersCount(ctx, communityID)
}

// ── reads ─────────────────────────────────────────────────────────────────────

func (s *CommunityService) GetByID(ctx context.Context, id int64) (*community.Community, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *CommunityService) GetBySlug(ctx context.Context, slug string) (*community.Community, error) {
	return s.repo.GetBySlug(ctx, slug)
}

// ── updates ───────────────────────────────────────────────────────────────────

func (s *CommunityService) Update(ctx context.Context, communityID, requesterID int64, req community.UpdateRequest) (*community.Community, error) {
	if err := s.RequireAdminOrOwner(ctx, communityID, requesterID); err != nil {
		return nil, err
	}
	c, err := s.repo.GetByID(ctx, communityID)
	if err != nil {
		return nil, err
	}
	if n := strings.TrimSpace(req.Name); n != "" {
		c.Name = n
	}
	c.Description = strings.TrimSpace(req.Description)
	if req.AvatarKey != "" {
		c.AvatarKey = req.AvatarKey
	}
	if req.BannerKey != "" {
		c.BannerKey = req.BannerKey
	}
	if err := s.repo.Update(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *CommunityService) ChangeType(ctx context.Context, communityID, requesterID int64, newType community.Type) (*community.Community, error) {
	c, err := s.repo.GetByID(ctx, communityID)
	if err != nil {
		return nil, err
	}
	// Only owner may change type.
	if c.OwnerID != requesterID {
		return nil, errs.ErrCommunityForbidden
	}
	c.Type = newType
	if err := s.repo.Update(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *CommunityService) SoftDelete(ctx context.Context, communityID, requesterID int64) error {
	c, err := s.repo.GetByID(ctx, communityID)
	if err != nil {
		return err
	}
	if c.OwnerID != requesterID {
		return errs.ErrCommunityForbidden
	}
	// TODO (Stage 2): if c.Type == community.TypeVideo, archive all videos.
	return s.repo.SoftDelete(ctx, communityID)
}

// ── listings ─────────────────────────────────────────────────────────────────

// ListManageable returns communities where userID is owner or admin.
func (s *CommunityService) ListManageable(ctx context.Context, userID int64) ([]*community.Community, error) {
	// We need communities where the user has role owner/admin.
	// The repository ListByOwner only covers ownership; we need a broader query.
	// Use a combined approach: list all member community IDs, then filter.
	// Since the repository doesn't expose a "list by admin or owner" directly,
	// use ListByOwner as a starting point and supplement if needed.
	// For now, do two queries and merge (acceptable at dev scale).
	adminRole := community.MemberRoleAdmin
	ownerRole := community.MemberRoleOwner

	var out []*community.Community

	// Owned communities (no pagination — manageable lists are typically small).
	owned, err := s.repo.ListByOwner(ctx, userID)
	if err != nil {
		return nil, err
	}
	out = append(out, owned...)

	// Communities where role = admin but user is NOT the owner.
	// ListMembers per community is expensive; instead walk member records for user.
	// TODO: add a Repository.ListByAdminUserID helper for efficiency at scale.
	// For dev scope, this approach is adequate.
	communityIDs, err := s.repo.ListMemberCommunityIDs(ctx, userID)
	if err != nil {
		return nil, err
	}
	_ = adminRole
	_ = ownerRole

	ownedSet := make(map[int64]bool, len(owned))
	for _, c := range owned {
		ownedSet[c.ID] = true
	}
	for _, cid := range communityIDs {
		if ownedSet[cid] {
			continue
		}
		m, err := s.repo.GetMember(ctx, cid, userID)
		if err != nil {
			continue
		}
		if m.Role == community.MemberRoleAdmin {
			c, err := s.repo.GetByID(ctx, cid)
			if err != nil {
				continue
			}
			out = append(out, c)
		}
	}
	return out, nil
}

func (s *CommunityService) Search(ctx context.Context, q string, t *community.Type, limit, offset int) ([]*community.Community, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	return s.repo.Search(ctx, q, t, limit, offset)
}

// ── membership ────────────────────────────────────────────────────────────────

func (s *CommunityService) Join(ctx context.Context, communityID, userID int64) error {
	c, err := s.repo.GetByID(ctx, communityID)
	if err != nil {
		return err
	}
	already, err := s.repo.IsMember(ctx, communityID, userID)
	if err != nil {
		return err
	}
	if already {
		return errs.ErrAlreadyCommunityMember
	}

	// For public communities the default role is "subscriber".
	// For private communities the role is "member".
	role := community.MemberRoleSubscriber
	if c.Visibility == community.VisibilityPrivate {
		role = community.MemberRoleMember
	}

	if err := s.repo.AddMember(ctx, &community.Member{
		CommunityID: communityID,
		UserID:      userID,
		Role:        role,
	}); err != nil {
		return err
	}
	return s.repo.IncrMembersCount(ctx, communityID)
}

func (s *CommunityService) Leave(ctx context.Context, communityID, userID int64) error {
	m, err := s.repo.GetMember(ctx, communityID, userID)
	if err != nil {
		if errors.Is(err, errs.ErrNotCommunityMember) {
			return errs.ErrNotCommunityMember
		}
		return err
	}
	if m.Role == community.MemberRoleOwner {
		return errs.ErrCannotRemoveCommunityOwner
	}
	if err := s.repo.RemoveMember(ctx, communityID, userID); err != nil {
		return err
	}
	return s.repo.DecrMembersCount(ctx, communityID)
}

func (s *CommunityService) ListMembers(ctx context.Context, communityID int64, roleFilter *community.MemberRole, limit, offset int) ([]*community.Member, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	return s.repo.ListMembers(ctx, communityID, roleFilter, limit, offset)
}

func (s *CommunityService) SetMemberRole(ctx context.Context, communityID, requesterID, targetUserID int64, role community.MemberRole) error {
	c, err := s.repo.GetByID(ctx, communityID)
	if err != nil {
		return err
	}
	if c.OwnerID != requesterID {
		return errs.ErrCommunityForbidden
	}
	if targetUserID == c.OwnerID {
		return errs.ErrCannotRemoveCommunityOwner
	}
	// Validate that target is already a member.
	if _, err := s.repo.GetMember(ctx, communityID, targetUserID); err != nil {
		return err
	}
	if role == community.MemberRoleOwner {
		// Transfer of ownership is not allowed via this endpoint.
		return errs.ErrCommunityForbidden
	}
	return s.repo.UpdateMemberRole(ctx, communityID, targetUserID, role)
}

// ── auth helper ───────────────────────────────────────────────────────────────

// RequireAdminOrOwner returns ErrCommunityForbidden if userID does not have
// admin or owner role in communityID. Called by PostService, VideoService,
// StoryService, and the messenger promote-to-community endpoint.
func (s *CommunityService) RequireAdminOrOwner(ctx context.Context, communityID, userID int64) error {
	ok, err := s.repo.IsAdminOrOwner(ctx, communityID, userID)
	if err != nil {
		return err
	}
	if !ok {
		return errs.ErrCommunityForbidden
	}
	return nil
}

// IsMember returns whether userID belongs to communityID (any role).
func (s *CommunityService) IsMember(ctx context.Context, communityID, userID int64) (bool, error) {
	return s.repo.IsMember(ctx, communityID, userID)
}

// ── counters ─────────────────────────────────────────────────────────────────

func (s *CommunityService) IncrVideosCount(ctx context.Context, communityID int64) error {
	return s.repo.IncrVideosCount(ctx, communityID)
}

func (s *CommunityService) DecrVideosCount(ctx context.Context, communityID int64) error {
	return s.repo.DecrVideosCount(ctx, communityID)
}

func (s *CommunityService) IncrPostsCount(ctx context.Context, communityID int64) error {
	return s.repo.IncrPostsCount(ctx, communityID)
}

func (s *CommunityService) DecrPostsCount(ctx context.Context, communityID int64) error {
	return s.repo.DecrPostsCount(ctx, communityID)
}

// ── compile-time interface check ─────────────────────────────────────────────

var _ community.Service = (*CommunityService)(nil)
