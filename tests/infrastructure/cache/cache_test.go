package cache

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	cache2 "github.com/unowned-22/api/internal/infrastructure/cache"
)

func TestMemoryCache_GetSetDelete(t *testing.T) {
	ctx := context.Background()
	c := cache2.NewMemoryCache()

	// 1. Get non-existent
	val, found, err := c.Get(ctx, 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Fatal("expected key 42 to not be found")
	}

	// 2. Set and Get
	err = c.Set(ctx, 42, 5, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	val, found, err = c.Get(ctx, 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Fatal("expected key 42 to be found")
	}
	if val != 5 {
		t.Errorf("expected version 5, got %d", val)
	}

	// 3. Expiration
	time.Sleep(60 * time.Millisecond)
	_, found, err = c.Get(ctx, 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Fatal("expected key 42 to be expired and not found")
	}

	// 4. Set, Delete and Get
	err = c.Set(ctx, 99, 10, 10*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = c.Delete(ctx, 99)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, found, err = c.Get(ctx, 99)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Fatal("expected key 99 to be deleted and not found")
	}
}

func TestRedisCache_GetSetDelete(t *testing.T) {
	ctx := context.Background()
	// Attempt to connect to a local Redis instance (default development port)
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer client.Close()

	// Skip test if Redis is not running
	pingCtx, pingCancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer pingCancel()
	if _, err := client.Ping(pingCtx).Result(); err != nil {
		t.Skip("local Redis is not running at localhost:6379; skipping RedisCache test")
	}

	c := cache2.NewRedisCache(client, "testapp")

	// Clean up key just in case
	_ = c.Delete(ctx, 100)

	// 1. Get non-existent
	val, found, err := c.Get(ctx, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Fatal("expected key 100 to not be found")
	}

	// 2. Set and Get
	err = c.Set(ctx, 100, 3, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	val, found, err = c.Get(ctx, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Fatal("expected key 100 to be found")
	}
	if val != 3 {
		t.Errorf("expected version 3, got %d", val)
	}

	// 3. Expiration
	time.Sleep(100 * time.Millisecond)
	_, found, err = c.Get(ctx, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Fatal("expected key 100 to be expired and not found")
	}

	// 4. Set, Delete and Get
	err = c.Set(ctx, 101, 7, 10*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = c.Delete(ctx, 101)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, found, err = c.Get(ctx, 101)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Fatal("expected key 101 to be deleted and not found")
	}
}
