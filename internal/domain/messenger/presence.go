package messenger

import "time"

type UserPresence struct {
	UserID     int64     `json:"user_id"`
	IsOnline   bool      `json:"is_online"`
	LastSeenAt time.Time `json:"last_seen_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
