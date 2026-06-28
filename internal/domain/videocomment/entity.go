package videocomment

import "time"

type Comment struct {
	ID         int64
	VideoID    int64
	UserID     int64
	ParentID   *int64
	Body       string
	LikesCount int64
	IsDeleted  bool
	IsLiked    bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
