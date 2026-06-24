package service

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"path"

	"github.com/google/uuid"
	"github.com/unowned-22/api/internal/domain/album"
	"github.com/unowned-22/api/internal/domain/photo"
	"github.com/unowned-22/api/internal/domain/storage"
	domainusersettings "github.com/unowned-22/api/internal/domain/usersettings"
	"github.com/unowned-22/api/internal/errs"
)

type photoService struct {
	photos   photo.Repository
	albums   album.Repository
	settings domainusersettings.Repository
	storage  storage.Storage
}

func NewPhotoService(photos photo.Repository, albums album.Repository, settings domainusersettings.Repository, storage storage.Storage) photo.Service {
	return &photoService{photos: photos, albums: albums, settings: settings, storage: storage}
}

func (s *photoService) Upload(ctx context.Context, userID int64, input photo.UploadInput) (*photo.Photo, error) {
	us, err := s.settings.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if us == nil || us.BucketName == "" {
		return nil, errs.ErrUserStorageNotProvisioned
	}
	if us.StorageUsedBytes+input.Size > us.StorageQuotaBytes {
		return nil, errs.ErrStorageQuotaExceeded
	}

	if input.AlbumID != nil {
		a, err := s.albums.GetByID(ctx, *input.AlbumID)
		if err != nil {
			return nil, err
		}
		if a == nil || a.UserID != userID {
			return nil, errs.ErrAlbumNotOwned
		}
	}

	// buffer input to detect image size and to upload
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, input.Reader); err != nil {
		return nil, fmt.Errorf("failed to read upload: %w", err)
	}
	data := buf.Bytes()
	// detect dimensions
	var widthPtr, heightPtr *int
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err == nil {
		w := cfg.Width
		h := cfg.Height
		widthPtr = &w
		heightPtr = &h
	}

	// extension from content type
	ext := "bin"
	switch input.ContentType {
	case "image/jpeg":
		ext = "jpg"
	case "image/png":
		ext = "png"
	case "image/webp":
		ext = "webp"
	}

	key := path.Join(fmt.Sprintf("photos/%d", userID), uuid.New().String()+"."+ext)

	url, err := s.storage.PutObject(ctx, us.BucketName, key, bytes.NewReader(data), int64(len(data)), input.ContentType)
	if err != nil {
		return nil, err
	}

	p := &photo.Photo{
		UserID:      userID,
		AlbumID:     input.AlbumID,
		DisplayName: input.Filename,
		StorageKey:  key,
		URL:         url,
		SizeBytes:   int64(len(data)),
		Width:       widthPtr,
		Height:      heightPtr,
		MimeType:    input.ContentType,
		Visibility:  photo.VisibilityEveryone,
		HiddenFrom:  []int64{},
	}

	if err := s.photos.Create(ctx, p); err != nil {
		return nil, err
	}

	if err := s.settings.IncrementUsedBytes(ctx, userID, p.SizeBytes); err != nil {
		return nil, err
	}

	return p, nil
}

func (s *photoService) GetPhoto(ctx context.Context, id int64, viewerID int64) (*photo.Photo, error) {
	p, err := s.photos.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, errs.ErrPhotoNotFound
	}
	if p.UserID == viewerID {
		return p, nil
	}
	if p.Visibility == photo.VisibilityNobody {
		return nil, errs.ErrPhotoAccessDenied
	}
	for _, hid := range p.HiddenFrom {
		if hid == viewerID {
			return nil, errs.ErrPhotoAccessDenied
		}
	}
	if p.Visibility == photo.VisibilityEveryone {
		return p, nil
	}
	// TODO: implement friends visibility
	return nil, errs.ErrPhotoAccessDenied
}

func (s *photoService) ListUserPhotos(ctx context.Context, ownerID int64, viewerID int64, limit, offset int) ([]*photo.Photo, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	return s.photos.ListByUser(ctx, ownerID, viewerID, limit, offset)
}

func (s *photoService) ListAlbumPhotos(ctx context.Context, albumID int64, viewerID int64, limit, offset int) ([]*photo.Photo, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	return s.photos.ListByAlbum(ctx, albumID, viewerID, limit, offset)
}

func (s *photoService) UpdateMeta(ctx context.Context, id int64, requesterID int64, displayName string, visibility photo.Visibility, hiddenFrom []int64) error {
	p, err := s.photos.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if p == nil {
		return errs.ErrPhotoNotFound
	}
	if p.UserID != requesterID {
		return errs.ErrPhotoNotOwned
	}
	if len(displayName) == 0 || len(displayName) > 255 {
		return fmt.Errorf("invalid display name")
	}
	return s.photos.UpdateMeta(ctx, id, displayName, visibility, hiddenFrom)
}

func (s *photoService) MoveToAlbum(ctx context.Context, id int64, requesterID int64, albumID *int64) error {
	p, err := s.photos.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if p == nil {
		return errs.ErrPhotoNotFound
	}
	if p.UserID != requesterID {
		return errs.ErrPhotoNotOwned
	}
	if albumID != nil {
		a, err := s.albums.GetByID(ctx, *albumID)
		if err != nil {
			return err
		}
		if a == nil || a.UserID != requesterID {
			return errs.ErrAlbumNotOwned
		}
	}
	return s.photos.MoveToAlbum(ctx, id, albumID)
}

func (s *photoService) Delete(ctx context.Context, id int64, requesterID int64) error {
	p, err := s.photos.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if p == nil {
		return errs.ErrPhotoNotFound
	}
	if p.UserID != requesterID {
		return errs.ErrPhotoNotOwned
	}

	// delete from storage (best-effort)
	us, _ := s.settings.GetByUserID(ctx, requesterID)
	if us != nil && us.BucketName != "" {
		_ = s.storage.DeleteObject(ctx, us.BucketName, p.StorageKey)
	}

	if err := s.photos.Delete(ctx, id); err != nil {
		return err
	}
	// decrement used bytes
	if err := s.settings.IncrementUsedBytes(ctx, requesterID, -p.SizeBytes); err != nil {
		return err
	}
	return nil
}
