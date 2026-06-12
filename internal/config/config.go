package config

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	AppPort string `envconfig:"APP_PORT" default:"8080"`
	AppEnv  string `envconfig:"APP_ENV" default:"development"`

	DBHost    string `envconfig:"DB_HOST"`
	DBPort    string `envconfig:"DB_PORT"`
	DBUser    string `envconfig:"DB_USER"`
	DBPass    string `envconfig:"DB_PASSWORD"`
	DBName    string `envconfig:"DB_NAME"`
	DBSSLMode string `envconfig:"DB_SSL_MODE" default:"disable"`

	JWTSecret string `envconfig:"JWT_SECRET"`
}

func Load() (*Config, error) {
	// 1. Load from .env if it exists
	_ = godotenv.Load()

	// 2. Load via envconfig into Config struct
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, fmt.Errorf("failed to process env variables: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) Validate() error {
	if strings.TrimSpace(c.AppPort) == "" {
		return fmt.Errorf("APP_PORT is required")
	}
	port, err := strconv.Atoi(c.AppPort)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("APP_PORT must be a valid TCP port")
	}

	required := map[string]string{
		"DB_HOST":    c.DBHost,
		"DB_PORT":    c.DBPort,
		"DB_USER":    c.DBUser,
		"DB_NAME":    c.DBName,
		"JWT_SECRET": c.JWTSecret,
	}
	for name, value := range required {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", name)
		}
	}

	dbPort, err := strconv.Atoi(c.DBPort)
	if err != nil || dbPort < 1 || dbPort > 65535 {
		return fmt.Errorf("DB_PORT must be a valid TCP port")
	}

	if strings.TrimSpace(c.DBSSLMode) == "" {
		return fmt.Errorf("DB_SSL_MODE is required")
	}

	return nil
}
