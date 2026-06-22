package bootstrap

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/unowned-22/api/internal/config"
	"github.com/unowned-22/api/internal/database"
	domainevent "github.com/unowned-22/api/internal/domain/event"
	"github.com/unowned-22/api/internal/infrastructure/mailer"
	"github.com/unowned-22/api/internal/infrastructure/queue"
	infrastorage "github.com/unowned-22/api/internal/infrastructure/storage"
	"github.com/unowned-22/api/internal/logger"
	postgresRepo "github.com/unowned-22/api/internal/repository/postgres"
	ws "github.com/unowned-22/api/internal/transport/ws"
	workerHandler "github.com/unowned-22/api/internal/worker/handler"
)

type Worker struct {
	Version   string
	Commit    string
	BuildDate string

	cfg      *config.Config
	pool     *pgxpool.Pool
	consumer *queue.AMQPConsumer
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
	notificationRepo := postgresRepo.NewNotificationRepository(pool)
	friendshipRepo := postgresRepo.NewFriendshipRepository(pool)
	hub := ws.NewHub()

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

	handlers := map[domainevent.Name]domainevent.Handler{
		domainevent.UserRegistered:         workerHandler.NewUserRegisteredHandler(smtpMailer, cfg.AppURL, cfg.AppName),
		domainevent.UserEmailVerified:      workerHandler.NewEmailVerifiedHandler(minioStorage, systemSettingsRepo, userSettingsRepo),
		domainevent.PasswordResetRequested: workerHandler.NewPasswordResetHandler(smtpMailer),
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
		domainevent.AccountDeactivated:          workerHandler.NewAuditHandler(auditRepo, domainevent.AccountDeactivated),
		domainevent.AccountActivated:            workerHandler.NewAuditHandler(auditRepo, domainevent.AccountActivated),
		// email send handler: deliver email.send events by calling SMTP mailer
		domainevent.EmailSend:             workerHandler.NewEmailSendHandler(smtpMailer),
		domainevent.StoryPublished:        workerHandler.NewStoryPublishedHandler(friendshipRepo, userSettingsRepo, notificationRepo, hub),
		domainevent.FriendRequestReceived: workerHandler.NewFriendRequestReceivedHandler(userSettingsRepo, notificationRepo, hub),
		domainevent.FriendRequestAccepted: workerHandler.NewFriendRequestAcceptedHandler(userSettingsRepo, notificationRepo, hub),
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

	w := &Worker{
		Version:   version,
		Commit:    commit,
		BuildDate: buildDate,
		cfg:       cfg,
		pool:      pool,
		consumer:  consumer,
	}
	return w, nil
}

func (w *Worker) Run() error {
	defer w.pool.Close()

	if err := w.consumer.Consume(); err != nil {
		w.consumer.Shutdown(context.Background())
		return fmt.Errorf("failed to start consuming: %w", err)
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

	logger.Log.Info("Graceful shutdown completed. Exiting.")
	return nil
}
