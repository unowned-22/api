package bootstrap

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/unowned-22/api/internal/auth"
	"github.com/unowned-22/api/internal/config"
	"github.com/unowned-22/api/internal/database"
	"github.com/unowned-22/api/internal/domain/token"
	"github.com/unowned-22/api/internal/domain/user"
	"github.com/unowned-22/api/internal/infrastructure/cache"
	"github.com/unowned-22/api/internal/infrastructure/mailer"
	"github.com/unowned-22/api/internal/infrastructure/queue"
	storageInfra "github.com/unowned-22/api/internal/infrastructure/storage"
	"github.com/unowned-22/api/internal/logger"
)

// InitInfrastructure initializes shared infrastructure components and returns them.
func InitInfrastructure(cfg *config.Config) (
	pool *pgxpool.Pool,
	minio *storageInfra.MinIOStorage,
	publisher *queue.AMQPPublisher,
	smtp *mailer.SMTPMailer,
	tokenManager token.ManagerExtended,
	tokenVersionCache user.TokenVersionCache,
	err error,
) {
	ctx := context.Background()
	pool, err = database.NewPostgresPool(ctx, cfg)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	minio, err = storageInfra.NewMinIOStorage(storageInfra.Config{
		Endpoint:        cfg.MinIOEndpoint,
		AccessKeyID:     cfg.MinIOAccessKey,
		SecretAccessKey: cfg.MinIOSecretKey,
		UseSSL:          cfg.MinIOUseSSL,
		Region:          cfg.MinIORegion,
	})
	if err != nil {
		pool.Close()
		return nil, nil, nil, nil, nil, nil, fmt.Errorf("failed to initialize MinIO storage: %w", err)
	}

	publisher, err = queue.New(queue.Config{URL: cfg.RabbitMQURL, Exchange: cfg.RabbitMQExchange})
	if err != nil {
		pool.Close()
		return nil, nil, nil, nil, nil, nil, fmt.Errorf("failed to initialize RabbitMQ publisher: %w", err)
	}

	smtp = mailer.New(mailer.Config{
		Host:     cfg.SMTPHost,
		Port:     cfg.SMTPPort,
		Username: cfg.SMTPUsername,
		Password: cfg.SMTPPassword,
		From:     cfg.SMTPFrom,
	})

	// token manager
	tm := auth.NewTokenManager(cfg.JWTSecret, cfg.JWTIssuer, cfg.JWTAudience, cfg.AccessTokenTTL)
	tokenManager = tm

	// cache
	if strings.TrimSpace(cfg.RedisURL) != "" {
		var rClient *redis.Client
		if strings.HasPrefix(cfg.RedisURL, "redis://") || strings.HasPrefix(cfg.RedisURL, "rediss://") {
			opt, err := redis.ParseURL(cfg.RedisURL)
			if err == nil {
				rClient = redis.NewClient(opt)
			}
		} else {
			rClient = redis.NewClient(&redis.Options{
				Addr: cfg.RedisURL,
			})
		}

		if rClient != nil {
			pingCtx, pingCancel := context.WithTimeout(ctx, 2*time.Second)
			_, pingErr := rClient.Ping(pingCtx).Result()
			pingCancel()

			if pingErr != nil {
				logger.Log.WithError(pingErr).Warnf("failed to connect to Redis at %s, falling back to in-memory cache", cfg.RedisURL)
				tokenVersionCache = cache.NewMemoryCache()
			} else {
				logger.Log.Infof("successfully connected to Redis at %s", cfg.RedisURL)
				tokenVersionCache = cache.NewRedisCache(rClient, cfg.AppName)
			}
		} else {
			tokenVersionCache = cache.NewMemoryCache()
		}
	} else {
		logger.Log.Info("REDIS_URL not configured, using in-memory cache")
		tokenVersionCache = cache.NewMemoryCache()
	}

	return pool, minio, publisher, smtp, tokenManager, tokenVersionCache, nil
}
