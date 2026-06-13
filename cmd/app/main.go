package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"syscall"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/spf13/cobra"

	"github.com/unowned-22/api/internal/auth"
	"github.com/unowned-22/api/internal/config"
	"github.com/unowned-22/api/internal/database"
	domainevent "github.com/unowned-22/api/internal/domain/event"
	domainmailer "github.com/unowned-22/api/internal/domain/mailer"

	"github.com/unowned-22/api/internal/infrastructure/mailer"
	"github.com/unowned-22/api/internal/infrastructure/queue"
	storageInfra "github.com/unowned-22/api/internal/infrastructure/storage"
	"github.com/unowned-22/api/internal/logger"
	postgresRepo "github.com/unowned-22/api/internal/repository/postgres"
	"github.com/unowned-22/api/internal/service"
	transportHttp "github.com/unowned-22/api/internal/transport/http"
	"github.com/unowned-22/api/internal/transport/http/handler"
	workerHandler "github.com/unowned-22/api/internal/worker/handler"
)

var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"

	migrateDownSteps  int
	migrateResetForce bool

	smtpMailer domainmailer.Mailer
)

var rootCmd = &cobra.Command{
	Use:   "app",
	Short: "REST API Application",
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start HTTP REST API server",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runServe()
	},
}

var workerCmd = &cobra.Command{
	Use:   "worker",
	Short: "Start RabbitMQ event consumer",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runWorker()
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show build information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Version: %s\n", Version)
		fmt.Printf("Commit: %s\n", Commit)
		fmt.Printf("Build Date: %s\n", BuildDate)
	},
}

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Database migrations management",
}

var migrateUpCmd = &cobra.Command{
	Use:   "up",
	Short: "Apply all database migrations",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runMigrateUp()
	},
}

var migrateDownCmd = &cobra.Command{
	Use:   "down",
	Short: "Rollback database migrations",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runMigrateDown(migrateDownSteps)
	},
}

var migrateResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Rollback all database migrations",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runMigrateReset(migrateResetForce)
	},
}

var migrateVersionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show current migration version",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runMigrateVersion()
	},
}

var migrateCreateCmd = &cobra.Command{
	Use:   "create [migration_name]",
	Short: "Create new up/down SQL migration files",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runMigrateCreate(args[0])
	},
}

func init() {
	migrateDownCmd.Flags().IntVar(&migrateDownSteps, "steps", 1, "number of migrations to roll back")
	migrateResetCmd.Flags().BoolVar(&migrateResetForce, "force", false, "confirm full migration rollback")

	migrateCmd.AddCommand(migrateUpCmd)
	migrateCmd.AddCommand(migrateDownCmd)
	migrateCmd.AddCommand(migrateResetCmd)
	migrateCmd.AddCommand(migrateVersionCmd)
	migrateCmd.AddCommand(migrateCreateCmd)

	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(workerCmd)
	rootCmd.AddCommand(migrateCmd)
	rootCmd.AddCommand(versionCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// runServe starts the REST API server with graceful shutdown.
func runServe() error {
	// 1. Config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// 2. Logger
	if err := logger.Init(); err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}
	logger.Log.Info("Logger successfully initialized")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 3. PostgreSQL
	pool, err := database.NewPostgresPool(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer func() {
		pool.Close()
		logger.Log.Info("PostgreSQL connection pool closed successfully")
	}()
	logger.Log.Info("PostgreSQL connection pool established")

	// 4. Repositories
	userRepo := postgresRepo.NewUserRepository(pool)
	refreshTokenRepo := postgresRepo.NewRefreshTokenRepository(pool)
	roleRepo := postgresRepo.NewRoleRepository(pool)
	permissionRepo := postgresRepo.NewPermissionRepository(pool)

	// 5. Object Storage
	minioStorage, err := storageInfra.NewMinIOStorage(storageInfra.Config{
		Endpoint:        cfg.MinIOEndpoint,
		AccessKeyID:     cfg.MinIOAccessKey,
		SecretAccessKey: cfg.MinIOSecretKey,
		UseSSL:          cfg.MinIOUseSSL,
		Region:          cfg.MinIORegion,
	})
	if err != nil {
		return fmt.Errorf("failed to initialize MinIO storage: %w", err)
	}

	// 6. Event Publisher (RabbitMQ)
	publisher, err := queue.New(queue.Config{
		URL:      cfg.RabbitMQURL,
		Exchange: cfg.RabbitMQExchange,
	})
	if err != nil {
		return fmt.Errorf("failed to initialize RabbitMQ publisher: %w", err)
	}
	defer func() {
		if err := publisher.Close(); err != nil {
			logger.Log.WithError(err).Error("Failed to close RabbitMQ publisher")
		}
	}()

	// 6. TokenManager
	tokenManager := auth.NewTokenManager(cfg.JWTSecret)

	// 7. Mailer
	smtpMailer = mailer.New(mailer.Config{
		Host:     cfg.SMTPHost,
		Port:     cfg.SMTPPort,
		Username: cfg.SMTPUsername,
		Password: cfg.SMTPPassword,
		From:     cfg.SMTPFrom,
	})

	// 8. Services
	authService := auth.NewAuthService(userRepo, refreshTokenRepo, roleRepo, tokenManager, smtpMailer, cfg.AppURL, cfg.AppName)
	passwordResetRepo := postgresRepo.NewPasswordResetRepository(pool)
	passwordResetService := service.NewPasswordResetService(userRepo, passwordResetRepo, refreshTokenRepo, smtpMailer, cfg.AppURL, cfg.AppName)
	userService := service.NewUserService(userRepo)
	permissionService := service.NewPermissionService(permissionRepo)
	healthService := service.NewHealthService(pool)

	// 9. Handlers
	authHandler := handler.NewAuthHandler(authService)
	passwordResetHandler := handler.NewPasswordResetHandler(passwordResetService)
	userHandler := handler.NewUserHandler(userService)
	adminHandler := handler.NewAdminHandler(userService, permissionService)
	healthHandler := handler.NewHealthHandler(healthService)
	uploadHandler := handler.NewUploadHandler(minioStorage, cfg.MinIOBucket)

	// 10. Router
	router := transportHttp.NewRouter(cfg, authHandler, userHandler, passwordResetHandler, adminHandler, uploadHandler, healthHandler, tokenManager, userService, permissionService)

	// 11. HTTP Server
	srv := &http.Server{
		Addr:              ":" + cfg.AppPort,
		Handler:           router,
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Graceful shutdown setup.
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(shutdown)

	logger.Log.WithFields(map[string]interface{}{
		"service": "api",
		"version": Version,
		"commit":  Commit,
		"env":     cfg.AppEnv,
		"port":    cfg.AppPort,
		"db_host": cfg.DBHost,
	}).Info("Starting application")

	errCh := make(chan error, 1)
	go func() {
		logger.Log.Infof("Starting server on port %s", cfg.AppPort)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("failed to start HTTP server: %w", err)
		}
	}()

	select {
	case sig := <-shutdown:
		logger.Log.Infof("Received termination signal (%s), initiating graceful shutdown...", sig)
	case err := <-errCh:
		logger.Log.Error(err)
		cancel()
		return err
	}

	cancel()

	ctxShut, cancelShut := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShut()

	// 1. Stop accepting new requests; wait for active ones to finish.
	if err := srv.Shutdown(ctxShut); err != nil {
		logger.Log.Errorf("HTTP server graceful shutdown failed: %v", err)
	} else {
		logger.Log.Info("HTTP server stopped accepting new requests")
	}

	logger.Log.Info("Graceful shutdown completed. Exiting.")
	return nil
}

// runWorker starts the RabbitMQ event consumer with graceful shutdown.
func runWorker() error {
	// 1. Config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// 2. Logger
	if err := logger.Init(); err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}
	logger.Log.Info("Logger successfully initialized")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 3. PostgreSQL (for future use, if needed by handlers)
	pool, err := database.NewPostgresPool(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer func() {
		pool.Close()
		logger.Log.Info("PostgreSQL connection pool closed successfully")
	}()
	logger.Log.Info("PostgreSQL connection pool established")

	// 4. SMTP Mailer
	smtpMailer = mailer.New(mailer.Config{
		Host:     cfg.SMTPHost,
		Port:     cfg.SMTPPort,
		Username: cfg.SMTPUsername,
		Password: cfg.SMTPPassword,
		From:     cfg.SMTPFrom,
	})

	// 5. Event Handlers
	handlers := map[domainevent.Name]domainevent.Handler{
		domainevent.UserRegistered:         workerHandler.NewUserRegisteredHandler(smtpMailer),
		domainevent.PasswordResetRequested: workerHandler.NewPasswordResetHandler(smtpMailer),
	}

	// 6. AMQP Consumer
	consumer, err := queue.NewConsumer(queue.ConsumerConfig{
		URL:      cfg.RabbitMQURL,
		Exchange: cfg.RabbitMQExchange,
		Queue:    cfg.RabbitMQQueue,
		Tag:      "worker",
	}, handlers)
	if err != nil {
		return fmt.Errorf("failed to create AMQP consumer: %w", err)
	}

	// 7. Start consuming
	if err := consumer.Consume(); err != nil {
		consumer.Shutdown(context.Background())
		return fmt.Errorf("failed to start consuming: %w", err)
	}

	logger.Log.WithFields(map[string]interface{}{
		"service":  "worker",
		"version":  Version,
		"commit":   Commit,
		"env":      cfg.AppEnv,
		"queue":    cfg.RabbitMQQueue,
		"exchange": cfg.RabbitMQExchange,
	}).Info("Starting RabbitMQ consumer")

	// 8. Graceful shutdown setup
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(shutdown)

	// Wait for signal
	sig := <-shutdown
	logger.Log.Infof("Received termination signal (%s), initiating graceful shutdown...", sig)

	// Shutdown consumer with timeout
	ctxShut, cancelShut := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShut()

	if err := consumer.Shutdown(ctxShut); err != nil {
		logger.Log.Errorf("Consumer graceful shutdown failed: %v", err)
	} else {
		logger.Log.Info("Consumer stopped gracefully")
	}

	logger.Log.Info("Graceful shutdown completed. Exiting.")
	return nil
}

func getMigrator(cfg *config.Config) (*migrate.Migrate, error) {
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		cfg.DBUser, cfg.DBPass, cfg.DBHost, cfg.DBPort, cfg.DBName, cfg.DBSSLMode,
	)
	m, err := migrate.New("file://migrations", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize migrator: %w", err)
	}
	return m, nil
}

func runMigrateUp() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	m, err := getMigrator(cfg)
	if err != nil {
		return err
	}
	defer m.Close()

	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			fmt.Println("No migrations to apply.")
			return nil
		}
		return fmt.Errorf("failed to apply migrations: %w", err)
	}
	fmt.Println("Migrations applied successfully!")
	return nil
}

func runMigrateDown(steps int) error {
	if steps < 1 {
		return fmt.Errorf("steps must be greater than zero")
	}
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	m, err := getMigrator(cfg)
	if err != nil {
		return err
	}
	defer m.Close()

	if err := m.Steps(-steps); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			fmt.Println("No migrations to rollback.")
			return nil
		}
		return fmt.Errorf("failed to rollback migrations: %w", err)
	}
	fmt.Printf("Rolled back %d migration(s) successfully!\n", steps)
	return nil
}

func runMigrateReset(force bool) error {
	if !force {
		return fmt.Errorf("refuse execution: use --force to rollback all migrations")
	}
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	m, err := getMigrator(cfg)
	if err != nil {
		return err
	}
	defer m.Close()

	if err := m.Down(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			fmt.Println("No migrations to rollback.")
			return nil
		}
		return fmt.Errorf("failed to reset migrations: %w", err)
	}
	fmt.Println("All migrations rolled back successfully!")
	return nil
}

func runMigrateVersion() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	m, err := getMigrator(cfg)
	if err != nil {
		return err
	}
	defer m.Close()

	version, dirty, err := m.Version()
	if err != nil {
		if errors.Is(err, migrate.ErrNilVersion) {
			fmt.Println("No migrations have been applied yet.")
			return nil
		}
		return fmt.Errorf("failed to read migration version: %w", err)
	}
	fmt.Printf("Current version: %d (dirty: %t)\n", version, dirty)
	return nil
}

func runMigrateCreate(name string) error {
	dir := "migrations"
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create migrations directory: %w", err)
	}

	files, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %w", err)
	}

	maxVer := 0
	re := regexp.MustCompile(`^(\d+)[_a-zA-Z0-9]*\.(up|down)\.sql$`)
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		matches := re.FindStringSubmatch(f.Name())
		if len(matches) > 1 {
			ver, err := strconv.Atoi(matches[1])
			if err == nil && ver > maxVer {
				maxVer = ver
			}
		}
	}

	nextVer := maxVer + 1
	prefix := fmt.Sprintf("%06d", nextVer)

	upFile := filepath.Join(dir, fmt.Sprintf("%s_%s.up.sql", prefix, name))
	downFile := filepath.Join(dir, fmt.Sprintf("%s_%s.down.sql", prefix, name))

	if err := os.WriteFile(upFile, []byte(""), 0644); err != nil {
		return fmt.Errorf("failed to create up migration: %w", err)
	}
	if err := os.WriteFile(downFile, []byte(""), 0644); err != nil {
		if removeErr := os.Remove(upFile); removeErr != nil {
			return fmt.Errorf("failed to create down migration: %w; additionally failed to remove up migration: %v", err, removeErr)
		}
		return fmt.Errorf("failed to create down migration: %w", err)
	}

	fmt.Printf("Created migration files:\n  %s\n  %s\n", upFile, downFile)
	return nil
}
