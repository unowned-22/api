package realtime

import (
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/unowned-22/api/internal/domain/notification"
	"github.com/unowned-22/api/internal/domain/usersettings"
	"github.com/unowned-22/api/internal/logger"
	"github.com/unowned-22/api/internal/pagination"
	ws "github.com/unowned-22/api/internal/transport/ws"
)

type stubNotificationRepo struct {
	last *notification.Notification
}

func (s *stubNotificationRepo) Create(ctx context.Context, n *notification.Notification) error {
	s.last = n
	return nil
}

func (s *stubNotificationRepo) CreateBatch(ctx context.Context, notifs []*notification.Notification) error {
	if len(notifs) > 0 {
		s.last = notifs[0]
	}
	return nil
}

func (s *stubNotificationRepo) ListByUser(ctx context.Context, userID int64, page pagination.Query) ([]*notification.Notification, int64, error) {
	return nil, 0, nil
}

func (s *stubNotificationRepo) MarkRead(ctx context.Context, userID int64, notificationID int64) error {
	return nil
}
func (s *stubNotificationRepo) MarkAllRead(ctx context.Context, userID int64) error { return nil }
func (s *stubNotificationRepo) CountUnread(ctx context.Context, userID int64) (int64, error) {
	return 0, nil
}

type stubUserSettingsRepo struct{}

func (s stubUserSettingsRepo) Create(ctx context.Context, settings *usersettings.UserSettings) error {
	return nil
}

func (s stubUserSettingsRepo) GetByUserID(ctx context.Context, userID int64) (*usersettings.UserSettings, error) {
	return nil, nil
}

func (s stubUserSettingsRepo) Update(ctx context.Context, settings *usersettings.UserSettings) error {
	return nil
}
func (s stubUserSettingsRepo) UpdateTheme(ctx context.Context, userID int64, theme json.RawMessage) error {
	return nil
}
func (s stubUserSettingsRepo) UpdateQuota(ctx context.Context, userID int64, quotaBytes int64) error {
	return nil
}
func (s stubUserSettingsRepo) UpdateBucketName(ctx context.Context, userID int64, bucketName string) error {
	return nil
}
func (s stubUserSettingsRepo) IncrementUsedBytes(ctx context.Context, userID int64, delta int64) error {
	return nil
}
func (s stubUserSettingsRepo) UpdateNotificationPreferences(ctx context.Context, userID int64, prefs json.RawMessage) error {
	return nil
}
func (s stubUserSettingsRepo) GetNotificationPreferences(ctx context.Context, userID int64) (json.RawMessage, error) {
	return nil, nil
}

func TestFriendRequestReceivedUsesFriendshipID(t *testing.T) {
	logger.Log = logrus.New()
	logger.Log.SetOutput(io.Discard)

	h := &FriendRequestReceivedHandler{
		usersetRepo: stubUserSettingsRepo{},
		notifRepo:   &stubNotificationRepo{},
		hub:         ws.NewHub(),
	}

	payload := []byte(`{"friendship_id":123,"requester_id":10,"addressee_id":20}`)
	if err := h.Handle(context.Background(), payload); err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	notif := h.notifRepo.(*stubNotificationRepo).last
	if notif == nil {
		t.Fatal("expected notification to be created")
	}
	if notif.EntityID != 123 {
		t.Fatalf("expected EntityID 123, got %d", notif.EntityID)
	}

	var p map[string]any
	if err := json.Unmarshal(notif.Payload, &p); err != nil {
		t.Fatalf("unmarshal payload failed: %v", err)
	}
	if got := p["friendship_id"]; got != float64(123) {
		t.Fatalf("expected friendship_id 123, got %v", got)
	}
	if _, ok := p["request_id"]; ok {
		t.Fatal("request_id should not be present")
	}
}
