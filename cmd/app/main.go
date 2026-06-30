package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/cobra"
	domainfriendship "github.com/unowned-22/api/internal/domain/friendship"
	domainmailer "github.com/unowned-22/api/internal/domain/mailer"
	domainusersettings "github.com/unowned-22/api/internal/domain/usersettings"
	"github.com/unowned-22/api/internal/errs"
	"github.com/unowned-22/api/internal/repository/postgres"

	"github.com/unowned-22/api/internal/bootstrap"
	"github.com/unowned-22/api/internal/config"
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
		app, err := bootstrap.NewApp()
		if err != nil {
			return err
		}

		err = runMigrateUp()
		if err != nil {
			return err
		}

		return app.Run()
	},
}

var workerCmd = &cobra.Command{
	Use:   "worker",
	Short: "Start RabbitMQ event consumer",
	RunE: func(cmd *cobra.Command, args []string) error {
		w, err := bootstrap.NewWorker(Version, Commit, BuildDate)
		if err != nil {
			return err
		}
		return w.Run()
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

var seedCmd = &cobra.Command{
	Use:   "seed",
	Short: "Seed development database using YAML fixtures",
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		cfg, err := config.Load()
		if err != nil {
			log.Fatalf("failed to load config: %v", err)
		}

		connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
			cfg.DBUser, cfg.DBPass, cfg.DBHost, cfg.DBPort, cfg.DBName, cfg.DBSSLMode,
		)

		pool, err := pgxpool.New(ctx, connStr)
		if err != nil {
			log.Fatalf("Database connection failed: %v", err)
		}
		defer pool.Close()

		userRepo := postgres.NewUserRepository(pool)
		userSettingsRepo := postgres.NewUserSettingsRepository(pool)
		friendshipRepo := postgres.NewFriendshipRepository(pool)

		log.Println("Parsing and building fixtures from YAML...")
		fixtures, err := bootstrap.LoadFixtures()
		if err != nil {
			log.Fatalf("Failed to build fixtures: %v", err)
		}

		usernameToID := make(map[string]int64)

		log.Printf("Inserting %d users into database...\n", len(fixtures.Users))
		for _, u := range fixtures.Users {
			err := userRepo.Create(ctx, u)
			if err != nil {
				if errors.Is(err, errs.ErrUserAlreadyExists) || errors.Is(err, errs.ErrUsernameAlreadyExists) {
					log.Printf("User %s already exists, пытаемся получить его ID...\n", u.Email)
					existingUser, getErr := userRepo.GetByEmail(ctx, u.Email)
					if getErr != nil {
						log.Fatalf("Failed to fetch existing user %s: %v", u.Email, getErr)
					}
					u.ID = existingUser.ID
				} else {
					log.Fatalf("Failed to seed user %s: %v", u.Email, err)
				}
			} else {
				err = userRepo.MarkEmailVerified(ctx, u.ID)
				if err != nil {
					log.Fatalf("Failed to mark email verified for user %s: %v", u.Email, err)
				}

				quotaBytes := int64(10 * 1024 * 1024 * 1024)
				bucketName := fmt.Sprintf("user-%d", u.ID)

				us := &domainusersettings.UserSettings{
					UserID:                  u.ID,
					StorageQuotaBytes:       quotaBytes,
					StorageUsedBytes:        0,
					BucketName:              bucketName,
					Theme:                   json.RawMessage(`{}`),
					NotificationPreferences: json.RawMessage(`{}`),
				}

				if err := userSettingsRepo.Create(ctx, us); err != nil {
					log.Fatalf("Failed to create user_settings for user %s: %v", u.Email, err)
				}
				log.Printf("Successfully created user: %s (ID: %d)\n", u.Email, u.ID)
			}

			usernameToID[u.Username] = u.ID
		}

		log.Printf("Establishing %d friendships...\n", len(fixtures.Friendships))
		for _, f := range fixtures.Friendships {
			requesterID, ok1 := usernameToID[f.RequesterUsername]
			addresseeID, ok2 := usernameToID[f.AddresseeUsername]

			if !ok1 || !ok2 {
				log.Printf("Warning: Skipping friendship between %s and %s. One of them wasn't found in database.\n", f.RequesterUsername, f.AddresseeUsername)
				continue
			}

			createdFriendship, err := friendshipRepo.Create(ctx, requesterID, addresseeID)
			if err != nil {
				if errors.Is(err, errs.ErrFriendshipAlreadyExist) {
					log.Printf("Friendship between %s and %s already exists, skipping.\n", f.RequesterUsername, f.AddresseeUsername)
					continue
				}
				log.Fatalf("Failed to create friendship between %s and %s: %v", f.RequesterUsername, f.AddresseeUsername, err)
			}

			if f.Status == "accepted" {
				_, err = friendshipRepo.UpdateStatus(ctx, createdFriendship.ID, domainfriendship.StatusAccepted)
				if err != nil {
					log.Fatalf("Failed to update friendship status to Accepted for ID %d: %v", createdFriendship.ID, err)
				}
				log.Printf("Successfully friended (Accepted): %s + %s\n", f.RequesterUsername, f.AddresseeUsername)
			} else {
				log.Printf("Successfully created friendship request (Pending): %s -> %s\n", f.RequesterUsername, f.AddresseeUsername)
			}
		}

		log.Println("Database seeding completely finished!")
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
	rootCmd.AddCommand(seedCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
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
