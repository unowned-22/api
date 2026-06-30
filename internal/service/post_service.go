package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/unowned-22/api/internal/domain/community"
	"github.com/unowned-22/api/internal/domain/communitypost"
	"github.com/unowned-22/api/internal/domain/event"
	"github.com/unowned-22/api/internal/domain/userpost"
	"github.com/unowned-22/api/internal/errs"
)

// AuthorType discriminates which table a post belongs to.
type AuthorType string

const (
	AuthorTypeUser      AuthorType = "user"
	AuthorTypeCommunity AuthorType = "community"
)

// MediaItemInput is the transport-agnostic input shape shared by both post types.
type MediaItemInput struct {
	Type       string
	StorageKey string
	Width      int
	Height     int
	DurationS  float64
}

// CreatePostRequest is the unified input for PostService.Create.
type CreatePostRequest struct {
	AuthorType  AuthorType
	CommunityID *int64 // required when AuthorType == AuthorTypeCommunity
	Text        string
	Media       []MediaItemInput
	Visibility  string // only meaningful for AuthorTypeUser
}

// PostResult is a union wrapper returned by PostService — exactly one of
// UserPost / CommunityPost is set, discriminated by SourceType.
type PostResult struct {
	SourceType    AuthorType
	UserPost      *userpost.Post
	CommunityPost *communitypost.Post
}

// PostService creates and deletes posts across both userpost and
// communitypost tables. It intentionally does NOT implement a generic
// "Post" domain interface — see AGENTS.md "Posts & feed (Stage 3)".
type PostService struct {
	userPostRepo      userpost.Repository
	communityPostRepo communitypost.Repository
	communitySvc      community.Service
	publisher         event.Publisher
}

func NewPostService(up userpost.Repository, cp communitypost.Repository, c community.Service, pub event.Publisher) *PostService {
	return &PostService{userPostRepo: up, communityPostRepo: cp, communitySvc: c, publisher: pub}
}

// Create validates input and persists either a user_posts or community_posts row.
func (s *PostService) Create(ctx context.Context, authorID int64, req CreatePostRequest) (*PostResult, error) {
	text := strings.TrimSpace(req.Text)
	if text == "" && len(req.Media) == 0 {
		return nil, fmt.Errorf("%w: post must have text or media", errs.ErrInvalidPostPayload)
	}

	switch req.AuthorType {
	case AuthorTypeUser, "":
		vis := userpost.VisibilityEveryone
		if req.Visibility != "" {
			vis = userpost.Visibility(req.Visibility)
		}
		if vis != userpost.VisibilityEveryone && vis != userpost.VisibilityFriends && vis != userpost.VisibilityPrivate {
			return nil, fmt.Errorf("%w: invalid visibility", errs.ErrInvalidPostPayload)
		}
		p := &userpost.Post{
			UserID:     authorID,
			Text:       text,
			Media:      toUserPostMedia(req.Media),
			Visibility: vis,
		}
		if err := s.userPostRepo.Create(ctx, p); err != nil {
			return nil, err
		}
		return &PostResult{SourceType: AuthorTypeUser, UserPost: p}, nil

	case AuthorTypeCommunity:
		if req.CommunityID == nil {
			return nil, fmt.Errorf("%w: community_id is required for author_type=community", errs.ErrInvalidPostPayload)
		}
		if err := s.communitySvc.RequireAdminOrOwner(ctx, *req.CommunityID, authorID); err != nil {
			return nil, err
		}
		p := &communitypost.Post{
			CommunityID:  *req.CommunityID,
			AuthorUserID: authorID,
			Text:         text,
			Media:        toCommunityPostMedia(req.Media),
		}
		if err := s.communityPostRepo.Create(ctx, p); err != nil {
			return nil, err
		}
		_ = s.communitySvc.IncrPostsCount(ctx, *req.CommunityID)

		payload, _ := json.Marshal(map[string]any{
			"community_id": p.CommunityID,
			"post_id":      p.ID,
			"text":         p.Text,
		})
		_ = s.publisher.Publish(ctx, event.Event{Name: event.CommunityPostPublished, Payload: payload})

		return &PostResult{SourceType: AuthorTypeCommunity, CommunityPost: p}, nil

	default:
		return nil, fmt.Errorf("%w: invalid author_type", errs.ErrInvalidPostPayload)
	}
}

// Delete soft-deletes a post. source distinguishes which table to look in.
// For community posts, the caller must be admin/owner of the community.
func (s *PostService) Delete(ctx context.Context, postID int64, source AuthorType, requesterID int64) error {
	switch source {
	case AuthorTypeUser:
		p, err := s.userPostRepo.GetByID(ctx, postID)
		if err != nil {
			return err
		}
		if p.UserID != requesterID {
			return errs.ErrPostForbidden
		}
		return s.userPostRepo.SoftDelete(ctx, postID)

	case AuthorTypeCommunity:
		p, err := s.communityPostRepo.GetByID(ctx, postID)
		if err != nil {
			return err
		}
		if err := s.communitySvc.RequireAdminOrOwner(ctx, p.CommunityID, requesterID); err != nil {
			return errs.ErrPostForbidden
		}
		if err := s.communityPostRepo.SoftDelete(ctx, postID); err != nil {
			return err
		}
		_ = s.communitySvc.DecrPostsCount(ctx, p.CommunityID)
		return nil

	default:
		return fmt.Errorf("%w: invalid source", errs.ErrInvalidPostPayload)
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

func toUserPostMedia(in []MediaItemInput) []userpost.MediaItem {
	out := make([]userpost.MediaItem, 0, len(in))
	for _, m := range in {
		out = append(out, userpost.MediaItem{
			Type: m.Type, StorageKey: m.StorageKey,
			Width: m.Width, Height: m.Height, DurationS: m.DurationS,
		})
	}
	return out
}

func toCommunityPostMedia(in []MediaItemInput) []communitypost.MediaItem {
	out := make([]communitypost.MediaItem, 0, len(in))
	for _, m := range in {
		out = append(out, communitypost.MediaItem{
			Type: m.Type, StorageKey: m.StorageKey,
			Width: m.Width, Height: m.Height, DurationS: m.DurationS,
		})
	}
	return out
}
