package service

import (
	"context"

	domainvideo "github.com/unowned-22/api/internal/domain/video"
	"github.com/unowned-22/api/internal/domain/videoplaylist"
	"github.com/unowned-22/api/internal/errs"
)

type VideoPlaylistService struct {
	repo      videoplaylist.Repository
	videoRepo domainvideo.Repository
}

func NewVideoPlaylistService(r videoplaylist.Repository, v domainvideo.Repository) *VideoPlaylistService {
	return &VideoPlaylistService{repo: r, videoRepo: v}
}
func (s *VideoPlaylistService) CreatePlaylist(ctx context.Context, userID int64, req videoplaylist.CreateRequest) (*videoplaylist.Playlist, error) {
	p := &videoplaylist.Playlist{UserID: userID, Title: req.Title, Description: req.Description, Visibility: req.Visibility}
	return p, s.repo.Create(ctx, p)
}
func (s *VideoPlaylistService) GetPlaylist(ctx context.Context, id int64, requesterID int64) (*videoplaylist.Playlist, error) {
	return s.repo.GetByID(ctx, id)
}
func (s *VideoPlaylistService) ListMyPlaylists(ctx context.Context, userID int64) ([]*videoplaylist.Playlist, error) {
	return s.repo.ListByUser(ctx, userID)
}
func (s *VideoPlaylistService) UpdatePlaylist(ctx context.Context, id int64, requesterID int64, req videoplaylist.UpdateRequest) (*videoplaylist.Playlist, error) {
	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if p.UserID != requesterID {
		return nil, errs.ErrPlaylistNotOwned
	}
	p.Title, p.Description, p.Visibility = req.Title, req.Description, req.Visibility
	return p, s.repo.Update(ctx, p)
}
func (s *VideoPlaylistService) DeletePlaylist(ctx context.Context, id int64, requesterID int64) error {
	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if p.UserID != requesterID {
		return errs.ErrPlaylistNotOwned
	}
	return s.repo.Delete(ctx, id)
}
func (s *VideoPlaylistService) AddVideoToPlaylist(ctx context.Context, playlistID, videoID, requesterID int64) error {
	return s.repo.AddItem(ctx, playlistID, videoID)
}
func (s *VideoPlaylistService) RemoveVideoFromPlaylist(ctx context.Context, playlistID, videoID, requesterID int64) error {
	return s.repo.RemoveItem(ctx, playlistID, videoID)
}
func (s *VideoPlaylistService) ListPlaylistItems(ctx context.Context, playlistID int64, requesterID int64, limit, offset int) ([]*videoplaylist.PlaylistItem, int, error) {
	return s.repo.ListItems(ctx, playlistID, limit, offset)
}

var _ videoplaylist.Service = (*VideoPlaylistService)(nil)
