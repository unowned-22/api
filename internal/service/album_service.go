package service

import (
	"context"
	"fmt"

	"github.com/unowned-22/api/internal/domain/album"
	"github.com/unowned-22/api/internal/domain/photo"
	"github.com/unowned-22/api/internal/errs"
)

type albumService struct {
	albums album.Repository
	photos photo.Repository
}

func NewAlbumService(albums album.Repository, photos photo.Repository) album.Service {
	return &albumService{albums: albums, photos: photos}
}

func (s *albumService) Create(ctx context.Context, userID int64, input album.CreateInput) (*album.Album, error) {
	if len(input.Title) < 1 || len(input.Title) > 128 {
		return nil, fmt.Errorf("invalid title")
	}
	if len(input.Description) > 512 {
		return nil, fmt.Errorf("description too long")
	}
	a := &album.Album{
		UserID:      userID,
		Title:       input.Title,
		Description: input.Description,
		Visibility:  input.Visibility,
		HiddenFrom:  input.HiddenFrom,
	}
	if err := s.albums.Create(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

func (s *albumService) Get(ctx context.Context, id int64, viewerID int64) (*album.Album, error) {
	a, err := s.albums.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, errs.ErrAlbumNotFound
	}
	if a.UserID == viewerID {
		return a, nil
	}
	if a.Visibility == album.VisibilityNobody {
		return nil, errs.ErrAlbumAccessDenied
	}
	for _, hid := range a.HiddenFrom {
		if hid == viewerID {
			return nil, errs.ErrAlbumAccessDenied
		}
	}
	if a.Visibility == album.VisibilityEveryone {
		return a, nil
	}
	// TODO: implement friends visibility
	return nil, errs.ErrAlbumAccessDenied
}

func (s *albumService) ListUserAlbums(ctx context.Context, ownerID int64, viewerID int64, limit, offset int) ([]*album.Album, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	return s.albums.ListByUser(ctx, ownerID, viewerID, limit, offset)
}

func (s *albumService) Update(ctx context.Context, id int64, requesterID int64, input album.UpdateInput) (*album.Album, error) {
	a, err := s.albums.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, errs.ErrAlbumNotFound
	}
	if a.UserID != requesterID {
		return nil, errs.ErrAlbumNotOwned
	}
	a.Title = input.Title
	a.Description = input.Description
	a.Visibility = input.Visibility
	a.HiddenFrom = input.HiddenFrom
	if err := s.albums.Update(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

func (s *albumService) Delete(ctx context.Context, id int64, requesterID int64) error {
	a, err := s.albums.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if a == nil {
		return errs.ErrAlbumNotFound
	}
	if a.UserID != requesterID {
		return errs.ErrAlbumNotOwned
	}
	return s.albums.Delete(ctx, id)
}

func (s *albumService) SetCover(ctx context.Context, albumID int64, requesterID int64, photoID *int64) error {
	a, err := s.albums.GetByID(ctx, albumID)
	if err != nil {
		return err
	}
	if a == nil {
		return errs.ErrAlbumNotFound
	}
	if a.UserID != requesterID {
		return errs.ErrAlbumNotOwned
	}
	if photoID != nil {
		p, err := s.photos.GetByID(ctx, *photoID)
		if err != nil {
			return err
		}
		if p == nil {
			return errs.ErrPhotoNotFound
		}
		if p.UserID != requesterID {
			return errs.ErrPhotoNotOwned
		}
	}
	return s.albums.SetCover(ctx, albumID, photoID)
}
