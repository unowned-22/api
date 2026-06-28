package videoplaylist

import "context"

type Repository interface {
	Create(ctx context.Context, p *Playlist) error
	GetByID(ctx context.Context, id int64) (*Playlist, error)
	ListByUser(ctx context.Context, userID int64) ([]*Playlist, error)
	Update(ctx context.Context, p *Playlist) error
	Delete(ctx context.Context, id int64) error
	AddItem(ctx context.Context, playlistID, videoID int64) error
	RemoveItem(ctx context.Context, playlistID, videoID int64) error
	ListItems(ctx context.Context, playlistID int64, limit, offset int) ([]*PlaylistItem, int, error)
	ItemExists(ctx context.Context, playlistID, videoID int64) (bool, error)
}

type Service interface {
	CreatePlaylist(ctx context.Context, userID int64, req CreateRequest) (*Playlist, error)
	GetPlaylist(ctx context.Context, id int64, requesterID int64) (*Playlist, error)
	ListMyPlaylists(ctx context.Context, userID int64) ([]*Playlist, error)
	UpdatePlaylist(ctx context.Context, id int64, requesterID int64, req UpdateRequest) (*Playlist, error)
	DeletePlaylist(ctx context.Context, id int64, requesterID int64) error
	AddVideoToPlaylist(ctx context.Context, playlistID, videoID, requesterID int64) error
	RemoveVideoFromPlaylist(ctx context.Context, playlistID, videoID, requesterID int64) error
	ListPlaylistItems(ctx context.Context, playlistID int64, requesterID int64, limit, offset int) ([]*PlaylistItem, int, error)
}

type CreateRequest struct {
	Title       string
	Description string
	Visibility  Visibility
}

type UpdateRequest struct {
	Title       string
	Description string
	Visibility  Visibility
}
