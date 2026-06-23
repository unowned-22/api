package story

import "context"

type StoryRepository interface {
	Create(ctx context.Context, s *Story) error
	// IsCloseFriend returns true if friendID is in ownerID's close friends list.
	IsCloseFriend(ctx context.Context, ownerID, friendID int64) (bool, error)
	ListActiveByUser(ctx context.Context, userID int64) ([]*Story, error)
	// ListFeed returns active stories visible to the given user. Filtering by
	// friends/close visibility is applied by the repository when possible.
	ListFeed(ctx context.Context, viewerID int64) ([]*Story, error)
	// ListExpired returns stories that have expired (expires_at <= now()).
	ListExpired(ctx context.Context) ([]*Story, error)
	// View persistence
	AddView(ctx context.Context, viewerID int64, storyID int64, slideIndex *int) error
	ListViewsByViewer(ctx context.Context, viewerID int64) (map[int64]map[int]bool, error)
	GetByID(ctx context.Context, id int64) (*Story, error)
	Delete(ctx context.Context, id int64) error

	// Likes and replies
	AddLike(ctx context.Context, viewerID int64, storyID int64) error
	RemoveLike(ctx context.Context, viewerID int64, storyID int64) error
	AddReply(ctx context.Context, viewerID int64, storyID int64, message string) error
	ListReplies(ctx context.Context, viewerID int64, storyID int64) ([]*Reply, error)
}

type StoryService interface {
	Publish(ctx context.Context, userID int64, slidesJSON []byte, visibility string, durationHours int, hiddenFrom []int64) (*Story, error)
	ListMyStories(ctx context.Context, userID int64) ([]*Story, error)
	Feed(ctx context.Context, userID int64) ([]*Story, error)
	AddView(ctx context.Context, viewerID int64, storyID int64, slideIndex *int) error
	ListViewsByViewer(ctx context.Context, viewerID int64) (map[int64]map[int]bool, error)
	Delete(ctx context.Context, userID int64, storyID int64) error

	// Likes and replies service-level
	Like(ctx context.Context, viewerID int64, storyID int64) error
	Unlike(ctx context.Context, viewerID int64, storyID int64) error
	Reply(ctx context.Context, viewerID int64, storyID int64, message string) error
	ListReplies(ctx context.Context, viewerID int64, storyID int64) ([]*Reply, error)
}
