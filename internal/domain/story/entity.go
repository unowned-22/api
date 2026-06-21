package story

import "time"

type Visibility string

const (
	VisibilityEveryone Visibility = "everyone"
	VisibilityFriends  Visibility = "friends"
	VisibilityClose    Visibility = "close"
)

// Story is a persisted, time-limited story published by a user.
// Slides is stored as opaque JSON (see Slides field comment) rather than
// being decomposed into relational tables.
type Story struct {
	ID                int64
	UserID            int64
	Visibility        Visibility
	DurationHours     int
	HiddenFromUserIDs []int64
	Slides            []byte // raw JSON array
	CreatedAt         time.Time
	ExpiresAt         time.Time
}
