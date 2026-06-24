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

	"encoding/json"
	"github.com/google/uuid"
	"github.com/rwcarlsen/goexif/exif"
	"github.com/unowned-22/api/internal/domain/album"
	"github.com/unowned-22/api/internal/domain/event"
	"github.com/unowned-22/api/internal/domain/photo"
	"github.com/unowned-22/api/internal/domain/storage"
	domainusersettings "github.com/unowned-22/api/internal/domain/usersettings"
	"github.com/unowned-22/api/internal/errs"
)

type photoService struct {
	photos       photo.Repository
	albums       album.Repository
	settings     domainusersettings.Repository
	storage      storage.Storage
	publisher    event.Publisher
	publicBucket string
}

func NewPhotoService(photos photo.Repository, albums album.Repository, settings domainusersettings.Repository, storage storage.Storage, publisher event.Publisher, publicBucket string) photo.Service {
	return &photoService{photos: photos, albums: albums, settings: settings, storage: storage, publisher: publisher, publicBucket: publicBucket}
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

	// parse EXIF for GPS if present
	var latPtr, lonPtr *float64
	var exifJSON []byte
	if x, err := exif.Decode(bytes.NewReader(data)); err == nil {
		if lat, lon, err := x.LatLong(); err == nil {
			latPtr = &lat
			lonPtr = &lon
		}
		// collect a small set of tags into JSON
		m := map[string]string{}
		if t, err := x.Get(exif.Model); err == nil {
			m["model"] = t.String()
		}
		if t, err := x.Get(exif.Make); err == nil {
			m["make"] = t.String()
		}
		if t, err := x.Get(exif.DateTimeOriginal); err == nil {
			m["datetime"] = t.String()
		}
		if len(m) > 0 {
			if b, err := json.Marshal(m); err == nil {
				exifJSON = b
			}
		}
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

	// Upload photos into the public bucket under photos/ prefix (like stories).
	bucket := s.publicBucket
	if bucket == "" {
		// fallback to user's bucket if public bucket not configured
		bucket = us.BucketName
	}
	url, err := s.storage.PutObject(ctx, bucket, key, bytes.NewReader(data), int64(len(data)), input.ContentType)
	if err != nil {
		return nil, err
	}

	p := &photo.Photo{
		UserID:       userID,
		AlbumID:      input.AlbumID,
		DisplayName:  input.Filename,
		StorageKey:   key,
		URL:          url,
		SizeBytes:    int64(len(data)),
		Width:        widthPtr,
		Height:       heightPtr,
		MimeType:     input.ContentType,
		Visibility:   photo.VisibilityEveryone,
		HiddenFrom:   []int64{},
		DeviceName:   input.DeviceName,
		DeviceOS:     input.DeviceOS,
		DeviceType:   input.DeviceType,
		Latitude:     latPtr,
		Longitude:    lonPtr,
		LocationName: input.LocationName,
		ExifData:     exifJSON,
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

	// delete from storage (best-effort) — prefer public bucket
	us, _ := s.settings.GetByUserID(ctx, requesterID)
	bucket := s.publicBucket
	if bucket == "" && us != nil {
		bucket = us.BucketName
	}
	if bucket != "" {
		_ = s.storage.DeleteObject(ctx, bucket, p.StorageKey)
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

func (s *photoService) LikePhoto(ctx context.Context, photoID int64, userID int64) error {
	p, err := s.photos.GetByID(ctx, photoID)
	if err != nil {
		return err
	}
	if p == nil {
		return errs.ErrPhotoNotFound
	}
	if err := s.photos.AddLike(ctx, userID, photoID); err != nil {
		return err
	}
	if p.UserID != userID && s.publisher != nil {
		payload, _ := json.Marshal(map[string]any{"photo_id": p.ID, "owner_id": p.UserID, "actor_id": userID})
		if err := s.publisher.Publish(ctx, event.Event{Name: event.PhotoLiked, Payload: payload}); err != nil {
			return err
		}
	}
	return nil
}

func (s *photoService) UnlikePhoto(ctx context.Context, photoID int64, userID int64) error {
	if err := s.photos.RemoveLike(ctx, userID, photoID); err != nil {
		return err
	}
	return nil
}
