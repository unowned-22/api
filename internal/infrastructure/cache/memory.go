package cache

import (
	"context"
	"sync"
	"time"

	"github.com/unowned-22/api/internal/domain/user"
)

type memoryItem struct {
	version   int
	expiresAt time.Time
}

type MemoryCache struct {
	mu    sync.RWMutex
	items map[int64]memoryItem
}

var _ user.TokenVersionCache = (*MemoryCache)(nil)

func NewMemoryCache() *MemoryCache {
	return &MemoryCache{
		items: make(map[int64]memoryItem),
	}
}

func (c *MemoryCache) Get(ctx context.Context, userID int64) (int, bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.items[userID]
	if !exists {
		return 0, false, nil
	}
	if time.Now().After(item.expiresAt) {
		return 0, false, nil
	}
	return item.version, true, nil
}

func (c *MemoryCache) Set(ctx context.Context, userID int64, version int, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[userID] = memoryItem{
		version:   version,
		expiresAt: time.Now().Add(ttl),
	}
	return nil
}

func (c *MemoryCache) Delete(ctx context.Context, userID int64) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.items, userID)
	return nil
}
