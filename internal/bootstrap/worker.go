package bootstrap

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/unowned-22/api/internal/config"
	"github.com/unowned-22/api/internal/database"
	domainevent "github.com/unowned-22/api/internal/domain/event"
	domainsearch "github.com/unowned-22/api/internal/domain/search"
	"github.com/unowned-22/api/internal/infrastructure/mailer"
	"github.com/unowned-22/api/internal/infrastructure/queue"
	infrasearch "github.com/unowned-22/api/internal/infrastructure/search"
	infrastorage "github.com/unowned-22/api/internal/infrastructure/storage"
	"github.com/unowned-22/api/internal/logger"
	postgresRepo "github.com/unowned-22/api/internal/repository/postgres"
	workerHandler "github.com/unowned-22/api/internal/worker/handler"
)

type Worker struct {
	Version   string
	Commit    string
	BuildDate string

	cfg             *config.Config
	pool            *pgxpool.Pool
	publisher       *queue.AMQPPublisher
	consumer        *queue.AMQPConsumer
	videoConsumer   *queue.AMQPConsumer
	userSearchIndex domainsearch.UserIndex
}

func NewWorker(version, commit, buildDate string) (*Worker, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	if err := logger.Init(); err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	ctx := context.Background()
	pool, err := database.NewPostgresPool(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	// close pool if any subsequent initialisation step fails
	defer func() {
		if err != nil {
			pool.Close()
		}
	}()

	smtpMailer := mailer.New(mailer.Config{
		Host:     cfg.SMTPHost,
		Port:     cfg.SMTPPort,
		Username: cfg.SMTPUsername,
		Password: cfg.SMTPPassword,
		From:     cfg.SMTPFrom,
	})

	auditRepo := postgresRepo.NewAuditRepository(pool)
	systemSettingsRepo := postgresRepo.NewSystemSettingsRepository(pool)
	userSettingsRepo := postgresRepo.NewUserSettingsRepository(pool)

	// initialize MinIO storage
	minioStorage, err := infrastorage.NewMinIOStorage(infrastorage.Config{
		Endpoint:        cfg.MinIOEndpoint,
		AccessKeyID:     cfg.MinIOAccessKey,
		SecretAccessKey: cfg.MinIOSecretKey,
		UseSSL:          cfg.MinIOUseSSL,
		Region:          cfg.MinIORegion,
		PublicEndpoint:  cfg.StoragePublicEndpoint,
		PublicBucket:    cfg.MinIOBucket,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize MinIO storage: %w", err)
	}

	publisher, err := queue.New(queue.Config{URL: cfg.RabbitMQURL, Exchange: cfg.RabbitMQExchange})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize RabbitMQ publisher: %w", err)
	}

	var userSearchIndex domainsearch.UserIndex
	if strings.TrimSpace(cfg.MeilisearchHost) != "" && strings.TrimSpace(cfg.MeilisearchAPIKey) != "" {
		idx, err := infrasearch.NewMeilisearchUserIndex(cfg.MeilisearchHost, cfg.MeilisearchAPIKey)
		if err != nil {
			logger.Log.WithError(err).Warn("meilisearch: failed to initialize, search will be unavailable")
		} else {
			userSearchIndex = idx
		}
	}

	userRepo := postgresRepo.NewUserRepository(pool)

	handlers := map[domainevent.Name]domainevent.Handler{
		domainevent.UserRegistered: workerHandler.NewUserRegisteredHandler(smtpMailer, cfg.AppURL, cfg.AppName),
		// EmailVerifiedHandler provisions storage/user_settings; the search
		// index handler additionally puts the now-confirmed user into the
		// "users" Meilisearch index. Both subscribe to the same event, so
		// they're combined via MultiHandler (the consumer only allows one
		// event.Handler per event.Name).
		domainevent.UserEmailVerified: workerHandler.NewMultiHandler(
			domainevent.UserEmailVerified,
			workerHandler.NewEmailVerifiedHandler(minioStorage, systemSettingsRepo, userSettingsRepo),
			workerHandler.NewUserSearchIndexHandler(userRepo, userSearchIndex, domainevent.UserEmailVerified),
		),
		domainevent.PasswordResetRequested: workerHandler.NewPasswordResetHandler(smtpMailer),
		// Re-syncs the search index whenever profile fields (name/username/avatar) change.
		domainevent.UserProfileUpdated: workerHandler.NewUserSearchIndexHandler(userRepo, userSearchIndex, domainevent.UserProfileUpdated),
		// Audit handlers
		domainevent.LoginSuccess:                workerHandler.NewAuditHandler(auditRepo, domainevent.LoginSuccess),
		domainevent.LoginFailed:                 workerHandler.NewAuditHandler(auditRepo, domainevent.LoginFailed),
		domainevent.Logout:                      workerHandler.NewAuditHandler(auditRepo, domainevent.Logout),
		domainevent.LogoutAll:                   workerHandler.NewAuditHandler(auditRepo, domainevent.LogoutAll),
		domainevent.VerificationSent:            workerHandler.NewAuditHandler(auditRepo, domainevent.VerificationSent),
		domainevent.EmailVerified:               workerHandler.NewAuditHandler(auditRepo, domainevent.EmailVerified),
		domainevent.PasswordResetRequestedAudit: workerHandler.NewAuditHandler(auditRepo, domainevent.PasswordResetRequestedAudit),
		domainevent.PasswordResetCompleted:      workerHandler.NewAuditHandler(auditRepo, domainevent.PasswordResetCompleted),
		domainevent.PasswordChanged:             workerHandler.NewAuditHandler(auditRepo, domainevent.PasswordChanged),
		domainevent.RefreshRotated:              workerHandler.NewAuditHandler(auditRepo, domainevent.RefreshRotated),
		domainevent.RefreshTokenReuseDetected:   workerHandler.NewAuditHandler(auditRepo, domainevent.RefreshTokenReuseDetected),
		domainevent.SessionRevoked:              workerHandler.NewAuditHandler(auditRepo, domainevent.SessionRevoked),
		// AccountDeactivated previously only fed the audit log; now it also
		// removes the user from search, since deactivated accounts should no
		// longer be discoverable. Combined the same way as UserEmailVerified above.
		domainevent.AccountDeactivated: workerHandler.NewMultiHandler(
			domainevent.AccountDeactivated,
			workerHandler.NewAuditHandler(auditRepo, domainevent.AccountDeactivated),
			workerHandler.NewUserSearchDeindexHandler(userSearchIndex, domainevent.AccountDeactivated),
		),
		domainevent.AccountActivated: workerHandler.NewAuditHandler(auditRepo, domainevent.AccountActivated),
		// email send handler: deliver email.send events by calling SMTP mailer
		domainevent.EmailSend: workerHandler.NewEmailSendHandler(smtpMailer),
	}

	consumer, err := queue.NewConsumer(queue.ConsumerConfig{
		URL:      cfg.RabbitMQURL,
		Exchange: cfg.RabbitMQExchange,
		Queue:    cfg.RabbitMQQueue,
		Tag:      "worker",
	}, handlers)
	if err != nil {
		return nil, fmt.Errorf("failed to create AMQP consumer: %w", err)
	}

	videoHandler := workerHandler.NewVideoProcessHandler(postgresRepo.NewVideoRepository(pool), minioStorage, cfg.MinIOBucket, publisher)
	videoConsumer, err := queue.NewConsumer(queue.ConsumerConfig{
		URL:      cfg.RabbitMQURL,
		Exchange: cfg.RabbitMQExchange,
		Queue:    cfg.VideoProcessQueue,
		Tag:      "worker-video",
	}, map[domainevent.Name]domainevent.Handler{
		domainevent.Name(cfg.VideoProcessQueue): videoHandler,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create video AMQP consumer: %w", err)
	}

	w := &Worker{
		Version:         version,
		Commit:          commit,
		BuildDate:       buildDate,
		cfg:             cfg,
		pool:            pool,
		publisher:       publisher,
		consumer:        consumer,
		videoConsumer:   videoConsumer,
		userSearchIndex: userSearchIndex,
	}
	return w, nil
}

func (w *Worker) Run() error {
	defer w.pool.Close()
	defer func() { _ = w.publisher.Close() }()

	if err := w.consumer.Consume(); err != nil {
		err := w.consumer.Shutdown(context.Background())
		if err != nil {
			return err
		}
		return fmt.Errorf("failed to start consuming: %w", err)
	}
	if err := w.videoConsumer.Consume(); err != nil {
		_ = w.consumer.Shutdown(context.Background())
		_ = w.videoConsumer.Shutdown(context.Background())
		return fmt.Errorf("failed to start video consuming: %w", err)
	}

	logger.Log.WithFields(map[string]interface{}{
		"service":  "worker",
		"version":  w.Version,
		"commit":   w.Commit,
		"env":      w.cfg.AppEnv,
		"queue":    w.cfg.RabbitMQQueue,
		"exchange": w.cfg.RabbitMQExchange,
	}).Info("Starting RabbitMQ consumer")

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(shutdown)

	sig := <-shutdown
	logger.Log.Infof("Received termination signal (%s), initiating graceful shutdown...", sig)

	ctxShut, cancelShut := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShut()

	if err := w.consumer.Shutdown(ctxShut); err != nil {
		logger.Log.Errorf("Consumer graceful shutdown failed: %v", err)
	} else {
		logger.Log.Info("Consumer stopped gracefully")
	}
	if err := w.videoConsumer.Shutdown(ctxShut); err != nil {
		logger.Log.Errorf("Video consumer graceful shutdown failed: %v", err)
	} else {
		logger.Log.Info("Video consumer stopped gracefully")
	}

	logger.Log.Info("Graceful shutdown completed. Exiting.")
	return nil
}
