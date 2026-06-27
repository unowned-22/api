package bootstrap

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/unowned-22/api/internal/config"
	"github.com/unowned-22/api/internal/infrastructure/queue"
	stor "github.com/unowned-22/api/internal/infrastructure/storage"
	"github.com/unowned-22/api/internal/logger"
	"github.com/unowned-22/api/internal/middleware"
	"github.com/unowned-22/api/internal/realtime"
	ws "github.com/unowned-22/api/internal/transport/ws"
	messengerworker "github.com/unowned-22/api/internal/worker"
	outboxworker "github.com/unowned-22/api/internal/worker/outbox"
)

type App struct {
	// Core runtime values
	Config  any
	DB      any
	Storage any
	Queue   any

	Router http.Handler
	Server *http.Server

	Repositories *Repositories
	Services     *Services
	Handlers     *Handlers
	Hub          *ws.Hub

	// internal pieces we need to shutdown
	loginLimiter, registerLimiter, forgotLimiter, resendLimiter *middleware.AuthRateLimiter
	publisher                                                   *queue.AMQPPublisher
	pool                                                        *pgxpool.Pool
	realtimeConsumer                                            *realtime.Consumer
	realtimeCancel                                              context.CancelFunc
	realtimeDone                                                chan struct{}
	outboxCancel                                                context.CancelFunc
}

// NewApp initializes application dependencies and returns an App ready to Run.
func NewApp() (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	if err := logger.Init(); err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	pool, minioStorage, publisher, smtpMailer, tokenManager, tokenVersionCache, err := InitInfrastructure(cfg)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			if publisher != nil {
				_ = publisher.Close()
			}
			if pool != nil {
				pool.Close()
			}
		}
	}()

	repos := InitRepositories(pool)
	hub := ws.NewHubWithPresence(repos.Presence, repos.Friendship)
	svcs := InitServices(cfg, pool, repos, tokenManager, smtpMailer, publisher, minioStorage, tokenVersionCache)
	handlers := InitHandlers(cfg, svcs, minioStorage, hub)
	realtimeConsumer, err := realtime.NewConsumer(cfg, repos.Friendship, repos.Story, repos.UserSettings, repos.Notification, hub, repos.Member)
	if err != nil {
		return nil, err
	}

	srv, loginLimiter, registerLimiter, forgotLimiter, resendLimiter := NewServer(cfg, handlers, tokenManager, svcs, tokenVersionCache)

	app := &App{
		Config:           cfg,
		DB:               pool,
		Storage:          minioStorage,
		Queue:            publisher,
		Router:           srv.Handler,
		Server:           srv,
		Repositories:     repos,
		Services:         svcs,
		Handlers:         handlers,
		Hub:              hub,
		realtimeConsumer: realtimeConsumer,

		loginLimiter:    loginLimiter,
		registerLimiter: registerLimiter,
		forgotLimiter:   forgotLimiter,
		resendLimiter:   resendLimiter,
		publisher:       publisher,
		pool:            pool,
	}

	// Start outbox worker to republish persisted events to the broker.
	if app.Repositories != nil && app.publisher != nil {
		ctxWorker, cancelWorker := context.WithCancel(context.Background())
		worker := outboxworker.NewWorker(app.Repositories.Outbox, app.publisher, outboxworker.RetryPolicy{MaxRetries: 5, Interval: 1 * time.Second}, 50)
		go worker.Start(ctxWorker)
		app.outboxCancel = cancelWorker
	}

	if app.realtimeConsumer != nil {
		ctxRealtime, cancelRealtime := context.WithCancel(context.Background())
		app.realtimeCancel = cancelRealtime
		app.realtimeDone = make(chan struct{})
		go func() {
			defer close(app.realtimeDone)
			if err := app.realtimeConsumer.Run(ctxRealtime); err != nil {
				logger.Log.WithError(err).Error("Realtime consumer stopped with error")
			}
		}()
	}

	// Start messenger workers (scheduled messages + disappearing messages).
	if app.Repositories != nil && app.Services != nil {
		ctxScheduled, cancelScheduled := context.WithCancel(context.Background())
		scheduledWorker := messengerworker.NewScheduledMessageWorker(app.Repositories.Message, app.Repositories.Member, app.Services.OutboxPublisher)
		go scheduledWorker.Run(ctxScheduled)

		ctxDisappearing, cancelDisappearing := context.WithCancel(context.Background())
		disappearingWorker := messengerworker.NewDisappearingMessageWorker(app.Repositories.Message, app.Repositories.Member, app.Hub)
		go disappearingWorker.Run(ctxDisappearing)

		prevCancel := app.outboxCancel
		app.outboxCancel = func() {
			if prevCancel != nil {
				prevCancel()
			}
			cancelScheduled()
			cancelDisappearing()
		}
	}

	// Start cleanup goroutine for expired stories (best-effort background job)
	if app.Repositories != nil && app.Storage != nil {
		if minIOStorage, ok := app.Storage.(*stor.MinIOStorage); ok {
			ctxCleanup, cancelCleanup := context.WithCancel(context.Background())
			StartCleanupExpired(ctxCleanup, app.Repositories.Story, minIOStorage, cfg.MinIOBucket, time.Duration(cfg.StoriesCleanupIntervalMinutes)*time.Minute)
			// attach cancel to shutdown via outboxCancel slot
			if app.outboxCancel == nil {
				app.outboxCancel = cancelCleanup
			} else {
				// wrap both cancels
				prev := app.outboxCancel
				app.outboxCancel = func() {
					prev()
					cancelCleanup()
				}
			}
		}
	}

	return app, nil
}

// Run starts the HTTP server and handles graceful shutdown.
func (a *App) Run() error {
	cfg, ok := a.Config.(*config.Config)
	if !ok {
		return fmt.Errorf("invalid config in App container")
	}

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(shutdown)

	errCh := make(chan error, 1)
	go func() {
		logger.Log.Infof("Starting server on port %s", cfg.AppPort)
		if err := a.Server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("failed to start HTTP server: %w", err)
		}
	}()

	select {
	case sig := <-shutdown:
		logger.Log.Infof("Received termination signal (%s), initiating graceful shutdown...", sig)
	case err := <-errCh:
		logger.Log.Error(err)
		return err
	}

	ctxShut, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := a.Server.Shutdown(ctxShut); err != nil {
		logger.Log.Errorf("HTTP server graceful shutdown failed: %v", err)
	} else {
		logger.Log.Info("HTTP server stopped accepting new requests")
	}

	// Stop limiters
	if a.loginLimiter != nil {
		a.loginLimiter.Stop()
	}
	if a.registerLimiter != nil {
		a.registerLimiter.Stop()
	}
	if a.forgotLimiter != nil {
		a.forgotLimiter.Stop()
	}
	if a.resendLimiter != nil {
		a.resendLimiter.Stop()
	}

	if a.realtimeCancel != nil {
		a.realtimeCancel()
	}
	if a.realtimeDone != nil {
		select {
		case <-a.realtimeDone:
			logger.Log.Info("Realtime consumer stopped gracefully")
		case <-ctxShut.Done():
			logger.Log.Warn("Realtime consumer shutdown timeout exceeded")
		}
	}

	if a.publisher != nil {
		err := a.publisher.Close()
		if err != nil {
			return err
		}
	}

	if a.outboxCancel != nil {
		a.outboxCancel()
	}

	if a.pool != nil {
		a.pool.Close()
		logger.Log.Info("PostgreSQL connection pool closed successfully")
	}

	logger.Log.Info("Graceful shutdown completed. Exiting.")
	return nil
}
