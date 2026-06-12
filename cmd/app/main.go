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
	"github.com/unowned-22/api/internal/logger"
	postgresRepo "github.com/unowned-22/api/internal/repository/postgres"
	"github.com/unowned-22/api/internal/service"
	transportHttp "github.com/unowned-22/api/internal/transport/http"
	"github.com/unowned-22/api/internal/transport/http/handler"
)

var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

var rootCmd = &cobra.Command{
	Use:   "app",
	Short: "REST API Application",
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start HTTP REST API server",
	Run: func(cmd *cobra.Command, args []string) {
		runServe()
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
		return runMigrateDown()
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
	migrateCmd.AddCommand(migrateUpCmd)
	migrateCmd.AddCommand(migrateDownCmd)
	migrateCmd.AddCommand(migrateVersionCmd)
	migrateCmd.AddCommand(migrateCreateCmd)

	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(migrateCmd)
	rootCmd.AddCommand(versionCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// runServe starts the REST API server with Graceful Shutdown
func runServe() {
	// 1. Config
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	// 2. Logger
	logger.Init()
	logger.Log.Info("Logger successfully initialized")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 3. PostgreSQL
	pool, err := database.NewPostgresPool(ctx, cfg)
	if err != nil {
		logger.Log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()
	logger.Log.Info("PostgreSQL connection pool established")

	// 4. Repositories
	userRepo := postgresRepo.NewUserRepository(pool)
	refreshTokenRepo := postgresRepo.NewRefreshTokenRepository(pool)
	roleRepo := postgresRepo.NewRoleRepository(pool)
	permissionRepo := postgresRepo.NewPermissionRepository(pool)

	// 5. TokenManager
	tokenManager := auth.NewTokenManager(cfg.JWTSecret)

	// 6. Services
	userService := service.NewUserService(userRepo, refreshTokenRepo, roleRepo, tokenManager)
	permissionService := service.NewPermissionService(permissionRepo)

	// 7. Handlers
	authHandler := handler.NewAuthHandler(userService)
	userHandler := handler.NewUserHandler(userService)
	adminHandler := handler.NewAdminHandler(userService, permissionService)

	// 8. Router
	router := transportHttp.NewRouter(authHandler, userHandler, adminHandler, tokenManager, userService, permissionService)

	// 9. HTTP Server
	srv := &http.Server{
		Addr:    ":" + cfg.AppPort,
		Handler: router,
	}

	// Graceful Shutdown setup
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	go func() {
		logger.Log.Infof("Starting server on port %s", cfg.AppPort)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Log.Fatalf("failed to start HTTP server: %v", err)
		}
	}()

	// Wait for termination signal
	sig := <-shutdown
	logger.Log.Infof("Received termination signal (%s), initiating graceful shutdown...", sig)

	ctxShut, cancelShut := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShut()

	// 1. Stop receiving new requests and wait for active requests to complete
	if err := srv.Shutdown(ctxShut); err != nil {
		logger.Log.Errorf("HTTP server graceful shutdown failed: %v", err)
	} else {
		logger.Log.Info("HTTP server stopped accepting new requests")
	}

	// 2. Close PostgreSQL connection pool (handled by defer but we can call it explicitly or log it here)
	pool.Close()
	logger.Log.Info("PostgreSQL connection pool closed successfully")

	logger.Log.Info("Graceful shutdown completed. Exiting.")
}

func getMigrator(cfg *config.Config) (*migrate.Migrate, error) {
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		cfg.DBUser,
		cfg.DBPass,
		cfg.DBHost,
		cfg.DBPort,
		cfg.DBName,
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

func runMigrateDown() error {
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
		return fmt.Errorf("failed to rollback migrations: %w", err)
	}

	fmt.Println("Migrations rolled back successfully!")
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
		return fmt.Errorf("failed to create down migration: %w", err)
	}

	fmt.Printf("Created migration files:\n  %s\n  %s\n", upFile, downFile)
	return nil
}
