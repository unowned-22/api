package bootstrap

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/unowned-22/api/internal/auth"
	"github.com/unowned-22/api/internal/config"
	"github.com/unowned-22/api/internal/database"
	"github.com/unowned-22/api/internal/domain/token"
	"github.com/unowned-22/api/internal/infrastructure/mailer"
	"github.com/unowned-22/api/internal/infrastructure/queue"
	storageInfra "github.com/unowned-22/api/internal/infrastructure/storage"
)

// InitInfrastructure initializes shared infrastructure components and returns them.
func InitInfrastructure(cfg *config.Config) (
	pool *pgxpool.Pool,
	minio *storageInfra.MinIOStorage,
	publisher *queue.AMQPPublisher,
	smtp *mailer.SMTPMailer,
	tokenManager token.ManagerExtended,
	err error,
) {
	ctx := context.Background()
	pool, err = database.NewPostgresPool(ctx, cfg)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("failed to connect to database: %w", err)
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
		return nil, nil, nil, nil, nil, fmt.Errorf("failed to initialize MinIO storage: %w", err)
	}

	publisher, err = queue.New(queue.Config{URL: cfg.RabbitMQURL, Exchange: cfg.RabbitMQExchange})
	if err != nil {
		pool.Close()
		return nil, nil, nil, nil, nil, fmt.Errorf("failed to initialize RabbitMQ publisher: %w", err)
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

	return pool, minio, publisher, smtp, tokenManager, nil
}
