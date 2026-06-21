package bootstrap

import (
	"context"
	"encoding/json"
	"time"

	stor "github.com/unowned-22/api/internal/infrastructure/storage"
	"github.com/unowned-22/api/internal/logger"
	postgresRepo "github.com/unowned-22/api/internal/repository/postgres"
)

// StartCleanupExpired starts a background goroutine that periodically deletes
// expired stories and removes associated objects from the given MinIO storage.
func StartCleanupExpired(ctx context.Context, repo *postgresRepo.StoryRepository, storage *stor.MinIOStorage, bucket string, interval time.Duration) {
	if interval <= 0 {
		interval = 10 * time.Minute
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				logger.Log.Info("story cleanup stopped")
				return
			case <-ticker.C:
				cleanupOnce(ctx, repo, storage, bucket)
			}
		}
	}()
}

func cleanupOnce(ctx context.Context, repo *postgresRepo.StoryRepository, storage *stor.MinIOStorage, bucket string) {
	sts, err := repo.ListExpired(ctx)
	if err != nil {
		logger.Log.Errorf("failed to list expired stories: %v", err)
		return
	}
	for _, s := range sts {
		// parse slides and collect media keys
		var slides []map[string]any
		if err := json.Unmarshal(s.Slides, &slides); err != nil {
			// if slides cannot be parsed, just delete the row
			if delErr := repo.Delete(ctx, s.ID); delErr != nil {
				logger.Log.Errorf("failed to delete expired story %d: %v", s.ID, delErr)
			}
			continue
		}
		// collect keys
		keys := make(map[string]struct{})
		for _, sl := range slides {
			if bg, ok := sl["background"].(map[string]any); ok {
				if kind, _ := bg["kind"].(string); kind == "media" {
					if urlv, _ := bg["url"].(string); urlv != "" {
						keys[urlv] = struct{}{}
					}
				}
			}
			if elems, ok := sl["elements"].([]any); ok {
				for _, e := range elems {
					if emap, ok := e.(map[string]any); ok {
						if typ, _ := emap["type"].(string); typ == "image" {
							if urlv, _ := emap["url"].(string); urlv != "" {
								keys[urlv] = struct{}{}
							}
						}
					}
				}
			}
		}
		// delete objects
		for k := range keys {
			if err := storage.DeleteObject(ctx, bucket, k); err != nil {
				logger.Log.Errorf("failed to delete object %s from bucket %s: %v", k, bucket, err)
			}
		}
		// delete story row
		if err := repo.Delete(ctx, s.ID); err != nil {
			logger.Log.Errorf("failed to delete expired story %d: %v", s.ID, err)
		} else {
			logger.Log.Infof("deleted expired story %d and %d objects", s.ID, len(keys))
		}
	}
}
