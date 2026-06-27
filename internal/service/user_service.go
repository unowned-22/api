package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"path"
	"strings"

	"github.com/unowned-22/api/internal/errs"
	"github.com/unowned-22/api/internal/validator"

	domainstorage "github.com/unowned-22/api/internal/domain/storage"
	"github.com/unowned-22/api/internal/domain/user"
	domainusersettings "github.com/unowned-22/api/internal/domain/usersettings"
)

type UserService struct {
	repo             user.UserRepository
	storage          domainstorage.Storage
	userSettingsRepo domainusersettings.Repository
	publicBucket     string
}

func NewUserService(repo user.UserRepository, storage domainstorage.Storage, userSettingsRepo domainusersettings.Repository, publicBucket string) *UserService {
	return &UserService{repo: repo, storage: storage, userSettingsRepo: userSettingsRepo, publicBucket: publicBucket}
}

func (s *UserService) GetProfile(ctx context.Context, userID int64) (*user.User, error) {
	return s.repo.GetByID(ctx, userID)
}

func (s *UserService) UpdateProfile(ctx context.Context, userID int64, fullName, username, phone string) error {
	if err := validator.Validate(struct {
		FullName string `validate:"required,min=2,max=100"`
		Username string `validate:"required,min=3,max=30,username"`
		Phone    string `validate:"omitempty,phone"`
	}{FullName: fullName, Username: username, Phone: phone}); err != nil {
		return err
	}

	return s.repo.UpdateProfile(ctx, userID, fullName, username, phone)
}

func (s *UserService) UploadAvatar(ctx context.Context, userID int64, file io.Reader, size int64, contentType string) (string, error) {
	allowed := map[string]string{"image/jpeg": "jpg", "image/png": "png", "image/webp": "webp"}
	ext, ok := allowed[contentType]
	if !ok {
		return "", fmt.Errorf("unsupported content type: %s", contentType)
	}
	if size > 5*1024*1024 {
		return "", fmt.Errorf("avatar exceeds maximum allowed size")
	}

	us, err := s.userSettingsRepo.GetByUserID(ctx, userID)
	if err != nil {
		return "", err
	}
	if us == nil || us.BucketName == "" {
		// Bucket provisioning is async (email_verified worker). Return a typed
		// sentinel so the transport layer can map this to 503 instead of 500.
		return "", errs.ErrUserStorageNotProvisioned
	}

	bucket := s.publicBucket
	if bucket == "" {
		bucket = us.BucketName
	}

	key := path.Join(fmt.Sprintf("user-%d", userID), "avatars", "avatar."+ext)

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, file); err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}
	if int64(buf.Len()) != size {
		size = int64(buf.Len())
	}

	urlPath, err := s.storage.PutObject(ctx, bucket, key, bytes.NewReader(buf.Bytes()), size, contentType)
	if err != nil {
		return "", err
	}

	if err := s.repo.UpdateAvatar(ctx, userID, urlPath); err != nil {
		return "", err
	}

	if err := s.userSettingsRepo.IncrementUsedBytes(ctx, userID, size); err != nil {
		return "", err
	}

	return urlPath, nil
}

func (s *UserService) UploadCover(ctx context.Context, userID int64, file io.Reader, size int64, contentType string) (string, error) {
	allowed := map[string]string{"image/jpeg": "jpg", "image/png": "png", "image/webp": "webp"}
	ext, ok := allowed[contentType]
	if !ok {
		return "", fmt.Errorf("unsupported content type: %s", contentType)
	}
	if size > 10*1024*1024 {
		return "", fmt.Errorf("cover exceeds maximum allowed size")
	}

	us, err := s.userSettingsRepo.GetByUserID(ctx, userID)
	if err != nil {
		return "", err
	}
	if us == nil || us.BucketName == "" {
		return "", errs.ErrUserStorageNotProvisioned
	}

	bucket := s.publicBucket
	if bucket == "" {
		bucket = us.BucketName
	}

	key := path.Join(fmt.Sprintf("user-%d", userID), "covers", "cover."+ext)

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, file); err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}
	if int64(buf.Len()) != size {
		size = int64(buf.Len())
	}

	url, err := s.storage.PutObject(ctx, bucket, key, bytes.NewReader(buf.Bytes()), size, contentType)
	if err != nil {
		return "", err
	}

	if err := s.repo.UpdateCover(ctx, userID, url); err != nil {
		return "", err
	}

	if err := s.userSettingsRepo.IncrementUsedBytes(ctx, userID, size); err != nil {
		return "", err
	}

	return url, nil
}

// DeleteAvatar removes the user's avatar object from storage and clears
// the avatar URL on the user record.
func (s *UserService) DeleteAvatar(ctx context.Context, userID int64) error {
	u, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if u == nil || u.AvatarURL == "" {
		return errs.ErrAvatarNotFound
	}

	bucket, err := s.resolveMediaBucket(ctx, userID)
	if err != nil {
		return err
	}

	key, ok := extractObjectKey(u.AvatarURL, bucket)
	if ok {
		if err := s.storage.DeleteObject(ctx, bucket, key); err != nil {
			return err
		}
	}

	return s.repo.UpdateAvatar(ctx, userID, "")
}

// DeleteCover removes the user's cover object from storage and clears
// the cover URL on the user record.
func (s *UserService) DeleteCover(ctx context.Context, userID int64) error {
	u, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if u == nil || u.CoverURL == "" {
		return errs.ErrCoverNotFound
	}

	bucket, err := s.resolveMediaBucket(ctx, userID)
	if err != nil {
		return err
	}

	key, ok := extractObjectKey(u.CoverURL, bucket)
	if ok {
		if err := s.storage.DeleteObject(ctx, bucket, key); err != nil {
			return err
		}
	}

	return s.repo.UpdateCover(ctx, userID, "")
}

// resolveMediaBucket returns the bucket avatars/covers are stored in,
// mirroring the logic used when uploading them.
func (s *UserService) resolveMediaBucket(ctx context.Context, userID int64) (string, error) {
	if s.publicBucket != "" {
		return s.publicBucket, nil
	}
	us, err := s.userSettingsRepo.GetByUserID(ctx, userID)
	if err != nil {
		return "", err
	}
	if us == nil || us.BucketName == "" {
		return "", errs.ErrUserStorageNotProvisioned
	}
	return us.BucketName, nil
}

// extractObjectKey pulls the object key out of a stored avatar/cover URL
// (either a permanent public URL or a presigned URL), given the bucket the
// object was stored in. Returns ok=false if the bucket segment can't be
// found, in which case callers should skip the storage delete rather than
// fail the whole operation (the DB pointer is still cleared).
func extractObjectKey(rawURL, bucket string) (string, bool) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", false
	}
	prefix := "/" + bucket + "/"
	idx := strings.Index(u.Path, prefix)
	if idx == -1 {
		return "", false
	}
	key := u.Path[idx+len(prefix):]
	if key == "" {
		return "", false
	}
	return key, true
}
