package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"path"
	"strings"

	"github.com/unowned-22/api/internal/domain/media"
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
	imageProcessor   *media.Processor
}

func NewUserService(
	repo user.UserRepository,
	storage domainstorage.Storage,
	userSettingsRepo domainusersettings.Repository,
	publicBucket string,
	imageProcessor *media.Processor,
) *UserService {
	return &UserService{
		repo:             repo,
		storage:          storage,
		userSettingsRepo: userSettingsRepo,
		publicBucket:     publicBucket,
		imageProcessor:   imageProcessor,
	}
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
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, file); err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}
	origBytes := buf.Bytes()
	if size != int64(len(origBytes)) {
		size = int64(len(origBytes))
	}

	// Detect format from actual bytes — ignore client-supplied Content-Type.
	f, err := media.DetectFormat(origBytes)
	if err != nil {
		return "", fmt.Errorf("unsupported content type: %w", err)
	}
	allowedAvatarFormats := map[media.Format]bool{
		media.FormatJPEG: true,
		media.FormatPNG:  true,
		media.FormatWebP: true,
	}
	if !allowedAvatarFormats[f] {
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
		return "", errs.ErrUserStorageNotProvisioned
	}

	bucket := s.publicBucket
	if bucket == "" {
		bucket = us.BucketName
	}

	ext := media.FormatExtension(f)
	key := path.Join(fmt.Sprintf("user-%d", userID), "avatars", "avatar."+ext)

	urlPath, err := s.storage.PutObject(ctx, bucket, key, bytes.NewReader(origBytes), size, contentType)
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

func (s *UserService) UploadCover(
	ctx context.Context,
	userID int64,
	file io.Reader,
	size int64,
	contentType string,
	crop user.CoverCrop,
) (*user.UserCover, error) {
	var origBuf bytes.Buffer
	if _, err := io.Copy(&origBuf, file); err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	origBytes := origBuf.Bytes()
	origSize := int64(len(origBytes))

	if origSize > 10*1024*1024 {
		return nil, fmt.Errorf("cover exceeds maximum allowed size")
	}

	// Detect format from actual bytes — ignore client-supplied Content-Type.
	f, err := media.DetectFormat(origBytes)
	if err != nil {
		return nil, fmt.Errorf("unsupported content type: %w", err)
	}
	allowedCoverFormats := map[media.Format]bool{
		media.FormatJPEG: true,
		media.FormatPNG:  true,
		media.FormatWebP: true,
		media.FormatAVIF: true,
		media.FormatHEIC: true,
	}
	if !allowedCoverFormats[f] {
		return nil, fmt.Errorf("unsupported content type: %s", contentType)
	}

	us, err := s.userSettingsRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if us == nil || us.BucketName == "" {
		return nil, errs.ErrUserStorageNotProvisioned
	}

	bucket := s.publicBucket
	if bucket == "" {
		bucket = us.BucketName
	}

	mobileCrop := media.CropRect{
		X:      int(crop.Mobile.X),
		Y:      int(crop.Mobile.Y),
		Width:  int(crop.Mobile.Width),
		Height: int(crop.Mobile.Height),
	}
	desktopCrop := media.CropRect{
		X:      int(crop.Desktop.X),
		Y:      int(crop.Desktop.Y),
		Width:  int(crop.Desktop.Width),
		Height: int(crop.Desktop.Height),
	}

	variants := []media.VariantSpec{
		{Name: "mobile", Crop: &mobileCrop, Format: media.FormatJPEG, Quality: 90},
		{Name: "desktop", Crop: &desktopCrop, Format: media.FormatJPEG, Quality: 90},
	}

	processed, err := s.imageProcessor.Process(ctx, origBytes, variants)
	if err != nil {
		return nil, fmt.Errorf("failed to process cover image: %w", err)
	}

	// Map results by name for safe lookup.
	byName := make(map[string]media.ProcessedVariant, len(processed))
	for _, pv := range processed {
		byName[pv.Name] = pv
	}

	base := path.Join(fmt.Sprintf("user-%d", userID), "covers")
	ext := media.FormatExtension(f)

	origKey := path.Join(base, "cover_original."+ext)
	mobileKey := path.Join(base, "cover_mobile.jpg")
	desktopKey := path.Join(base, "cover_desktop.jpg")

	origURL, err := s.storage.PutObject(ctx, bucket, origKey, bytes.NewReader(origBytes), origSize, contentType)
	if err != nil {
		return nil, fmt.Errorf("failed to store original cover: %w", err)
	}

	mobileData := byName["mobile"].Data
	mobileURL, err := s.storage.PutObject(ctx, bucket, mobileKey, bytes.NewReader(mobileData), int64(len(mobileData)), "image/jpeg")
	if err != nil {
		return nil, fmt.Errorf("failed to store mobile cover: %w", err)
	}

	desktopData := byName["desktop"].Data
	desktopURL, err := s.storage.PutObject(ctx, bucket, desktopKey, bytes.NewReader(desktopData), int64(len(desktopData)), "image/jpeg")
	if err != nil {
		return nil, fmt.Errorf("failed to store desktop cover: %w", err)
	}

	if err := s.repo.UpdateCover(ctx, userID, mobileURL, desktopURL, origURL); err != nil {
		return nil, err
	}

	totalBytes := origSize + int64(len(mobileData)) + int64(len(desktopData))
	if err := s.userSettingsRepo.IncrementUsedBytes(ctx, userID, totalBytes); err != nil {
		return nil, err
	}

	return &user.UserCover{
		CoverURL:        origURL,
		CoverMobileURL:  mobileURL,
		CoverDesktopURL: desktopURL,
	}, nil
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

// DeleteCover removes the user's cover objects from storage and clears
// the cover URLs on the user record.
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

	return s.repo.UpdateCover(ctx, userID, "", "", "")
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

// extractObjectKey pulls the object key out of a stored avatar/cover URL.
// Returns ok=false if the bucket segment can't be found; callers should
// skip the storage delete rather than fail (the DB pointer is still cleared).
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
