package messenger

import "time"

type WhoCanMessage string

const (
	WhoCanMessageEveryone WhoCanMessage = "everyone"
	WhoCanMessageFriends  WhoCanMessage = "friends"
	WhoCanMessageNobody   WhoCanMessage = "nobody"
)

type MessengerPrivacySettings struct {
	UserID        int64         `json:"user_id"`
	WhoCanMessage WhoCanMessage `json:"who_can_message"`
	UpdatedAt     time.Time     `json:"updated_at"`
}
