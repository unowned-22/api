package community

import "context"

// Repository is the persistence contract.
// Implementations live in internal/repository/postgres.
type Repository interface {
	// Community CRUD
	Create(ctx context.Context, c *Community) error
	GetByID(ctx context.Context, id int64) (*Community, error)
	GetBySlug(ctx context.Context, slug string) (*Community, error)
	Update(ctx context.Context, c *Community) error
	SoftDelete(ctx context.Context, id int64) error

	// Listings / search
	ListByOwner(ctx context.Context, ownerID int64) ([]*Community, error)
	ListByType(ctx context.Context, t Type, limit, offset int) ([]*Community, error)
	Search(ctx context.Context, q string, t *Type, limit, offset int) ([]*Community, error)

	// Members
	AddMember(ctx context.Context, m *Member) error
	RemoveMember(ctx context.Context, communityID, userID int64) error
	UpdateMemberRole(ctx context.Context, communityID, userID int64, role MemberRole) error
	GetMember(ctx context.Context, communityID, userID int64) (*Member, error)
	ListMembers(ctx context.Context, communityID int64, roleFilter *MemberRole, limit, offset int) ([]*Member, error)
	// ListMemberCommunityIDs returns all community IDs the user belongs to
	// (any role). Used to build personalised feed queries.
	ListMemberCommunityIDs(ctx context.Context, userID int64) ([]int64, error)
	IsMember(ctx context.Context, communityID, userID int64) (bool, error)
	IsAdminOrOwner(ctx context.Context, communityID, userID int64) (bool, error)

	// Denormalised counter helpers — always called inside the same transaction
	// as the triggering write (join/leave/post/video).
	IncrMembersCount(ctx context.Context, communityID int64) error
	DecrMembersCount(ctx context.Context, communityID int64) error
	IncrPostsCount(ctx context.Context, communityID int64) error
	DecrPostsCount(ctx context.Context, communityID int64) error
	IncrVideosCount(ctx context.Context, communityID int64) error
	DecrVideosCount(ctx context.Context, communityID int64) error
}

// Service is the application-layer contract consumed by HTTP handlers
// and by other services that need community authority checks.
type Service interface {
	// Create registers a new community owned by ownerID.
	Create(ctx context.Context, ownerID int64, req CreateRequest) (*Community, error)

	// GetByID returns a community, respecting soft-delete.
	GetByID(ctx context.Context, id int64) (*Community, error)

	// GetBySlug is the slug-based lookup (for URLs / mentions).
	GetBySlug(ctx context.Context, slug string) (*Community, error)

	// Update patches mutable fields. Only owner or admin may call this;
	// the service enforces the check.
	Update(ctx context.Context, communityID, requesterID int64, req UpdateRequest) (*Community, error)

	// ChangeType switches the community type (e.g. video → general).
	// Only the owner may change the type.
	ChangeType(ctx context.Context, communityID, requesterID int64, newType Type) (*Community, error)

	// SoftDelete marks the community as deleted. Only the owner may do this.
	// If type=video, all community videos are set to status=archived.
	SoftDelete(ctx context.Context, communityID, requesterID int64) error

	// ListManageable returns communities where userID is owner or admin.
	// Used by the "post as" selector in the UI.
	ListManageable(ctx context.Context, userID int64) ([]*Community, error)

	// Search performs a full-text search across community names.
	Search(ctx context.Context, q string, t *Type, limit, offset int) ([]*Community, error)

	// Join adds the user to the community.
	// For public communities the default role is "subscriber";
	// for private communities the role is "member".
	Join(ctx context.Context, communityID, userID int64) error

	// Leave removes the user from the community.
	Leave(ctx context.Context, communityID, userID int64) error

	// ListMembers returns paginated members, optionally filtered by role.
	ListMembers(ctx context.Context, communityID int64, roleFilter *MemberRole, limit, offset int) ([]*Member, error)

	// SetMemberRole promotes or demotes a member. Only the owner may call this.
	SetMemberRole(ctx context.Context, communityID, requesterID, targetUserID int64, role MemberRole) error

	// AddMember adds targetUserID directly with the given role (owner only).
	// Used internally by MessengerService.PromoteToCommunity to bulk-migrate
	// conversation_members → community_members (Stage 5). Unlike Join, this
	// is not self-service — the actor must be the community owner.
	AddMember(ctx context.Context, communityID, actorID, targetUserID int64, role MemberRole) error

	// RequireAdminOrOwner returns ErrCommunityForbidden if userID is not
	// an admin or owner of communityID. Used by PostService, VideoService,
	// StoryService, and the messenger promote-to-community endpoint.
	RequireAdminOrOwner(ctx context.Context, communityID, userID int64) error

	// IsMember returns whether userID belongs to communityID (any role).
	// Used by StoryService to gate access to community story feeds (Stage 6).
	IsMember(ctx context.Context, communityID, userID int64) (bool, error)

	// IncrVideosCount / DecrVideosCount maintain the denormalised
	// communities.videos_count counter. Called by VideoService on
	// upload/delete (Stage 2).
	IncrVideosCount(ctx context.Context, communityID int64) error
	DecrVideosCount(ctx context.Context, communityID int64) error

	// IncrPostsCount / DecrPostsCount maintain the denormalised
	// communities.posts_count counter. Called by PostService (Stage 3).
	IncrPostsCount(ctx context.Context, communityID int64) error
	DecrPostsCount(ctx context.Context, communityID int64) error
}

// CreateRequest carries validated input for Service.Create.
type CreateRequest struct {
	Type        Type
	Visibility  Visibility
	Name        string
	Slug        string
	Description string

	// LinkConversationID is set internally by MessengerService.PromoteToCommunity
	// to link an existing conversation instead of auto-creating a new one
	// (Stage 5). Not part of the public HTTP request DTO — callers from
	// transport must never set this.
	LinkConversationID *int64
}

// UpdateRequest carries mutable fields for Service.Update.
type UpdateRequest struct {
	Name        string
	Description string
	AvatarKey   string
	BannerKey   string
}
