package dto

type SendRequest struct {
	AddresseeID int64 `json:"addressee_id" validate:"required,gt=0"`
}

type FriendshipResponse struct {
	ID          int64  `json:"id"`
	RequesterID int64  `json:"requester_id"`
	AddresseeID int64  `json:"addressee_id"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}
