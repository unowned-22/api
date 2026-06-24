package dto

type PhotoResponse struct {
	ID            int64    `json:"id"`
	AlbumID       *int64   `json:"album_id,omitempty"`
	DisplayName   string   `json:"display_name"`
	URL           string   `json:"url"`
	SizeBytes     int64    `json:"size_bytes"`
	Width         *int     `json:"width,omitempty"`
	Height        *int     `json:"height,omitempty"`
	MimeType      string   `json:"mime_type"`
	Visibility    string   `json:"visibility"`
	LikesCount    int      `json:"likes_count"`
	CommentsCount int      `json:"comments_count"`
	IsLiked       bool     `json:"is_liked"`
	DeviceName    string   `json:"device_name,omitempty"`
	DeviceOS      string   `json:"device_os,omitempty"`
	DeviceType    string   `json:"device_type,omitempty"`
	Latitude      *float64 `json:"latitude,omitempty"`
	Longitude     *float64 `json:"longitude,omitempty"`
	LocationName  string   `json:"location_name,omitempty"`
	ExifData      []byte   `json:"exif_data,omitempty"`
	CreatedAt     string   `json:"created_at"`
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
