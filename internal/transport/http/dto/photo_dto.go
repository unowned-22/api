package dto

type PhotoResponse struct {
	ID          int64  `json:"id"`
	AlbumID     *int64 `json:"album_id,omitempty"`
	DisplayName string `json:"display_name"`
	URL         string `json:"url"`
	SizeBytes   int64  `json:"size_bytes"`
	Width       *int   `json:"width,omitempty"`
	Height      *int   `json:"height,omitempty"`
	MimeType    string `json:"mime_type"`
	Visibility  string `json:"visibility"`
	CreatedAt   string `json:"created_at"`
}

type UploadPhotoRequest struct {
	AlbumID *int64 `json:"album_id,omitempty"`
}

type UpdatePhotoRequest struct {
	DisplayName string  `json:"display_name"`
	Visibility  string  `json:"visibility"`
	HiddenFrom  []int64 `json:"hidden_from"`
}

type MovePhotoRequest struct {
	AlbumID *int64 `json:"album_id"`
}
