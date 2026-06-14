package cache

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/unowned-22/api/internal/domain/user"
)

type RedisCache struct {
	client *redis.Client
	prefix string
}

var _ user.TokenVersionCache = (*RedisCache)(nil)

func NewRedisCache(client *redis.Client, prefix string) *RedisCache {
	return &RedisCache{
		client: client,
		prefix: prefix,
	}
}

func (c *RedisCache) key(userID int64) string {
	return fmt.Sprintf("%s:token_version:%d", c.prefix, userID)
}

func (c *RedisCache) Get(ctx context.Context, userID int64) (int, bool, error) {
	val, err := c.client.Get(ctx, c.key(userID)).Result()
	if err == redis.Nil {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	version, err := strconv.Atoi(val)
	if err != nil {
		return 0, false, err
	}
	return version, true, nil
}

func (c *RedisCache) Set(ctx context.Context, userID int64, version int, ttl time.Duration) error {
	return c.client.Set(ctx, c.key(userID), version, ttl).Err()
}

func (c *RedisCache) Delete(ctx context.Context, userID int64) error {
	return c.client.Del(ctx, c.key(userID)).Err()
}
