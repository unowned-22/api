package dto

type AlbumResponse struct {
	ID          int64   `json:"id"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Visibility  string  `json:"visibility"`
	CoverURL    *string `json:"cover_url,omitempty"`
	PhotoCount  int     `json:"photo_count"`
	CreatedAt   string  `json:"created_at"`
}

type CreateAlbumRequest struct {
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Visibility  string  `json:"visibility"`
	HiddenFrom  []int64 `json:"hidden_from"`
}

type UpdateAlbumRequest struct {
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Visibility  string  `json:"visibility"`
	HiddenFrom  []int64 `json:"hidden_from"`
}

type SetAlbumCoverRequest struct {
	PhotoID *int64 `json:"photo_id"`
}
