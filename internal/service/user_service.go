package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path"

	"github.com/unowned-22/api/internal/errs"
	"github.com/unowned-22/api/internal/validator"

	domainstorage "github.com/unowned-22/api/internal/domain/storage"
	"github.com/unowned-22/api/internal/domain/user"
	domainusersettings "github.com/unowned-22/api/internal/domain/usersettings"
)

// UserService implements domain/user.UserService.
type UserService struct {
	repo             user.UserRepository
	storage          domainstorage.Storage
	userSettingsRepo domainusersettings.Repository
	publicBucket     string
}

// NewUserService creates a new instance of UserService.
func NewUserService(repo user.UserRepository, storage domainstorage.Storage, userSettingsRepo domainusersettings.Repository, publicBucket string) *UserService {
	return &UserService{repo: repo, storage: storage, userSettingsRepo: userSettingsRepo, publicBucket: publicBucket}
}

// GetProfile returns the full user record (including role) by ID.
func (s *UserService) GetProfile(ctx context.Context, userID int64) (*user.User, error) {
	return s.repo.GetByID(ctx, userID)
}

// ListUsers returns users for the requested page and limit along with total count.
func (s *UserService) ListUsers(ctx context.Context, page int, limit int) ([]*user.User, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	offset := (page - 1) * limit
	items, err := s.repo.List(ctx, offset, limit)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.repo.Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// UpdateProfile validates and updates the user's profile fields.
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
	// validate content type
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

	// Avatars are stored in the global public bucket under a per-user prefix.
	bucket := s.publicBucket
	if bucket == "" {
		// Fallback to user's bucket if public bucket not configured.
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

	url, err := s.storage.PutObject(ctx, bucket, key, bytes.NewReader(buf.Bytes()), size, contentType)
	if err != nil {
		return "", err
	}

	if err := s.repo.UpdateAvatar(ctx, userID, url); err != nil {
		return "", err
	}

	if err := s.userSettingsRepo.IncrementUsedBytes(ctx, userID, size); err != nil {
		return "", err
	}

	return url, nil
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

	// Covers are stored in the global public bucket under a per-user prefix.
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

// Compile-time check that UserService satisfies the domain contract.
var _ user.UserService = (*UserService)(nil)
