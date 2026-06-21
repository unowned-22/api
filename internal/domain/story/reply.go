package story

import "time"

type Reply struct {
	ID        int64     `json:"id"`
	StoryID   int64     `json:"story_id"`
	ViewerID  int64     `json:"viewer_id"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}
