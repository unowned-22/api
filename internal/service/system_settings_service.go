package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/unowned-22/api/internal/domain/systemsettings"
)

// systemSettingsService implements systemsettings.Service
type systemSettingsService struct {
	repo systemsettings.Repository
}

// NewSystemSettingsService creates a new service.
func NewSystemSettingsService(repo systemsettings.Repository) systemsettings.Service {
	return &systemSettingsService{repo: repo}
}

var allowedKeys = map[string]struct{}{
	"default_storage_quota_bytes": {},
	"default_bucket_policy":       {},
	"theme":                       {},
}

func (s *systemSettingsService) GetAll(ctx context.Context) ([]*systemsettings.Setting, error) {
	return s.repo.GetAll(ctx)
}

func (s *systemSettingsService) GetByKey(ctx context.Context, key string) (*systemsettings.Setting, error) {
	if _, ok := allowedKeys[key]; !ok {
		return nil, fmt.Errorf("invalid setting key")
	}
	return s.repo.GetByKey(ctx, key)
}

func (s *systemSettingsService) Update(ctx context.Context, key string, value json.RawMessage) error {
	if _, ok := allowedKeys[key]; !ok {
		return fmt.Errorf("invalid setting key")
	}
	// basic validation: ensure value is valid JSON
	var tmp interface{}
	if err := json.Unmarshal(value, &tmp); err != nil {
		return fmt.Errorf("invalid JSON value: %w", err)
	}
	return s.repo.Upsert(ctx, key, value)
}
