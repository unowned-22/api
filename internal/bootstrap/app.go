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
	"github.com/unowned-22/api/internal/infrastructure/mailer"
	"github.com/unowned-22/api/internal/infrastructure/queue"
	"github.com/unowned-22/api/internal/logger"
	"github.com/unowned-22/api/internal/middleware"
)

type App struct {
	Version   string
	Commit    string
	BuildDate string

	cfg *config.Config

	pool      *pgxpool.Pool
	publisher *queue.AMQPPublisher
	smtp      *mailer.SMTPMailer
	server    *http.Server
	// limiters need stopping
	loginLimiter, registerLimiter, forgotLimiter, resendLimiter *middleware.AuthRateLimiter
}

// NewApp initializes application dependencies and returns an App ready to Run.
func NewApp(version, commit, buildDate string) (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	if err := logger.Init(); err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}
	// Initialize infra
	pool, minioStorage, publisher, smtpMailer, tokenManager, err := InitInfrastructure(cfg)
	if err != nil {
		return nil, err
	}

	// Repositories
	repos := InitRepositories(pool)

	// Services
	svcs := InitServices(cfg, pool, repos, tokenManager, smtpMailer, publisher)

	// Handlers
	handlers := InitHandlers(cfg, svcs, minioStorage)

	// Server and limiters
	srv, loginLimiter, registerLimiter, forgotLimiter, resendLimiter := NewServer(cfg, handlers, tokenManager, svcs)

	app := &App{
		Version:         version,
		Commit:          commit,
		BuildDate:       buildDate,
		cfg:             cfg,
		pool:            pool,
		publisher:       publisher,
		smtp:            smtpMailer,
		server:          srv,
		loginLimiter:    loginLimiter,
		registerLimiter: registerLimiter,
		forgotLimiter:   forgotLimiter,
		resendLimiter:   resendLimiter,
	}

	return app, nil
}

// Run starts the HTTP server and handles graceful shutdown.
func (a *App) Run() error {
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(shutdown)

	errCh := make(chan error, 1)
	go func() {
		logger.Log.Infof("Starting server on port %s", a.cfg.AppPort)
		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
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

	if err := a.server.Shutdown(ctxShut); err != nil {
		logger.Log.Errorf("HTTP server graceful shutdown failed: %v", err)
	} else {
		logger.Log.Info("HTTP server stopped accepting new requests")
	}

	// Stop limiters
	a.loginLimiter.Stop()
	a.registerLimiter.Stop()
	a.forgotLimiter.Stop()
	a.resendLimiter.Stop()

	if a.publisher != nil {
		a.publisher.Close()
	}

	if a.pool != nil {
		// placeholder: real pool.Close called via database package type
	}

	logger.Log.Info("Graceful shutdown completed. Exiting.")
	return nil
}
