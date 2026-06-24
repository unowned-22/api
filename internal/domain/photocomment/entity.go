package photocomment

import "time"

type Comment struct {
	ID           int64
	PhotoID      int64
	AuthorID     int64
	ParentID     *int64
	Body         string
	IsDeleted    bool
	LikesCount   int
	RepliesCount int
	CreatedAt    time.Time
	UpdatedAt    time.Time

	// Denormalized / derived for responses
	Author  *Author
	IsLiked bool
}

type Author struct {
	ID        int64
	FullName  string
	Username  string
	AvatarURL string
}
