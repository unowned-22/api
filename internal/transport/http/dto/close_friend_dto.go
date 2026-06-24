package dto

type AddCloseFriendRequest struct {
	FriendID int64 `json:"friend_id" validate:"required"`
}

type CloseFriendResponse struct {
	FriendID int64 `json:"friend_id"`
}
