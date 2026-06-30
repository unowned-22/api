package community

import "time"

// Type describes what kind of community this is.
// Stored as VARCHAR(32) — not a Postgres ENUM — so new types
// can be added by migration without a table rewrite.
type Type string

const (
	TypeGeneral Type = "general" // ordinary group / blog
	TypeVideo   Type = "video"   // video channel (replaces video_channels)
	TypeMusic   Type = "music"
	TypeNews    Type = "news"
	TypeGaming  Type = "gaming"
	TypeBlog    Type = "blog"
)

// Visibility controls who can see and join the community.
type Visibility string

const (
	VisibilityPublic  Visibility = "public"
	VisibilityPrivate Visibility = "private"
)

// Community is the core aggregate root.
type Community struct {
	ID          int64
	OwnerID     int64
	Type        Type
	Visibility  Visibility
	Name        string
	Slug        string // unique, used in URLs / mentions
	Description string
	AvatarKey   string
	BannerKey   string

	// Denormalised counters — kept in sync transactionally.
	MembersCount     int64
	PostsCount       int64
	SubscribersCount int64 // mirrors old video_channels.subscribers_count
	VideosCount      int64 // mirrors old video_channels.videos_count

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time // soft-delete
}

// MemberRole defines the access level of a community participant.
// Intentionally a separate type from messenger.MemberRole —
// domain packages must not import each other (see AGENTS.md §1).
type MemberRole string

const (
	// MemberRoleOwner is the creator; exactly one per community.
	// Ownership can be transferred via TransferOwnership.
	MemberRoleOwner MemberRole = "owner"

	// MemberRoleAdmin can post from the community, moderate,
	// and change community settings, but cannot delete the
	// community or transfer ownership.
	MemberRoleAdmin MemberRole = "admin"

	// MemberRoleMember belongs to a private group; can read
	// but cannot post from the community's identity.
	MemberRoleMember MemberRole = "member"

	// MemberRoleSubscriber is used for public channels/feeds:
	// the user follows the community (sees posts in their feed)
	// but is not a full member with chat access.
	// Equivalent to the old video_subscriptions role.
	MemberRoleSubscriber MemberRole = "subscriber"
)

// Member is a single community ↔ user relationship record.
type Member struct {
	CommunityID int64
	UserID      int64
	Role        MemberRole
	JoinedAt    time.Time
}
