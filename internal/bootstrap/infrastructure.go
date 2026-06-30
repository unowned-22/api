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
	domainsearch "github.com/unowned-22/api/internal/domain/search"
	"github.com/unowned-22/api/internal/domain/token"
	"github.com/unowned-22/api/internal/domain/user"
	"github.com/unowned-22/api/internal/infrastructure/cache"
	"github.com/unowned-22/api/internal/infrastructure/mailer"
	"github.com/unowned-22/api/internal/infrastructure/queue"
	infrasearch "github.com/unowned-22/api/internal/infrastructure/search"
	storageInfra "github.com/unowned-22/api/internal/infrastructure/storage"
	"github.com/unowned-22/api/internal/logger"
)

func InitInfrastructure(cfg *config.Config) (
	pool *pgxpool.Pool,
	minio *storageInfra.MinIOStorage,
	publisher *queue.AMQPPublisher,
	smtp *mailer.SMTPMailer,
	tokenManager token.ManagerExtended,
	tokenVersionCache user.TokenVersionCache,
	userSearchIndex domainsearch.UserIndex,
	err error,
) {
	ctx := context.Background()
	pool, err = database.NewPostgresPool(ctx, cfg)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	minio, err = storageInfra.NewMinIOStorage(storageInfra.Config{
		Endpoint:        cfg.MinIOEndpoint,
		AccessKeyID:     cfg.MinIOAccessKey,
		SecretAccessKey: cfg.MinIOSecretKey,
		UseSSL:          cfg.MinIOUseSSL,
		Region:          cfg.MinIORegion,
		PublicEndpoint:  cfg.StoragePublicEndpoint,
		PublicBucket:    cfg.MinIOBucket,
	})
	if err != nil {
		pool.Close()
		return nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("failed to initialize MinIO storage: %w", err)
	}

	if minio != nil {
		if err := minio.CreateBucket(context.Background(), cfg.MinIOBucket); err != nil {
			logger.Log.WithError(err).Warnf("failed to create bucket %s", cfg.MinIOBucket)
		}
		publicPolicy := `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":["*"]},"Action":["s3:GetObject"],"Resource":["arn:aws:s3:::` + cfg.MinIOBucket + `/*"]}]}`
		if err := minio.ApplyBucketPolicy(context.Background(), cfg.MinIOBucket, publicPolicy); err != nil {
			logger.Log.WithError(err).Warnf("failed to apply public policy to bucket %s", cfg.MinIOBucket)
		} else {
			logger.Log.Infof("applied public-read policy to bucket %s", cfg.MinIOBucket)
		}
	}

	publisher, err = queue.New(queue.Config{URL: cfg.RabbitMQURL, Exchange: cfg.RabbitMQExchange})
	if err != nil {
		pool.Close()
		return nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("failed to initialize RabbitMQ publisher: %w", err)
	}

	smtp = mailer.New(mailer.Config{
		Host:     cfg.SMTPHost,
		Port:     cfg.SMTPPort,
		Username: cfg.SMTPUsername,
		Password: cfg.SMTPPassword,
		From:     cfg.SMTPFrom,
	})

	tm := auth.NewTokenManager(cfg.JWTSecret, cfg.JWTIssuer, cfg.JWTAudience, cfg.AccessTokenTTL)
	tokenManager = tm

	if strings.TrimSpace(cfg.RedisURL) != "" {
		var rClient *redis.Client
		if strings.HasPrefix(cfg.RedisURL, "redis://") || strings.HasPrefix(cfg.RedisURL, "rediss://") {
			opt, err := redis.ParseURL(cfg.RedisURL)
			if err != nil {
				logger.Log.WithError(err).
					Warn("invalid REDIS_URL")
			} else {
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
				_ = rClient.Close()
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

	var idx domainsearch.UserIndex
	if strings.TrimSpace(cfg.MeilisearchHost) != "" && strings.TrimSpace(cfg.MeilisearchAPIKey) != "" {
		meiliIndex, err := infrasearch.NewMeilisearchUserIndex(cfg.MeilisearchHost, cfg.MeilisearchAPIKey)
		if err != nil {
			logger.Log.WithError(err).Warn("meilisearch: failed to initialize, search will be unavailable")
		} else {
			idx = meiliIndex
		}
	}

	return pool, minio, publisher, smtp, tokenManager, tokenVersionCache, idx, nil
}
