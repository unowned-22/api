package messenger

import "time"

type BlockedUser struct {
	BlockerID int64     `json:"blocker_id"`
	BlockedID int64     `json:"blocked_id"`
	CreatedAt time.Time `json:"created_at"`
}
