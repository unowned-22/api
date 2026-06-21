package story

import "context"

type StoryRepository interface {
	Create(ctx context.Context, s *Story) error
	ListActiveByUser(ctx context.Context, userID int64) ([]*Story, error)
	GetByID(ctx context.Context, id int64) (*Story, error)
	Delete(ctx context.Context, id int64) error
}

type StoryService interface {
	Publish(ctx context.Context, userID int64, slidesJSON []byte, visibility string, durationHours int, hiddenFrom []int64) (*Story, error)
	ListMyStories(ctx context.Context, userID int64) ([]*Story, error)
	Delete(ctx context.Context, userID int64, storyID int64) error
}
