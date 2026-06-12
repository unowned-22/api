package config

import (
	"fmt"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	AppPort string `envconfig:"APP_PORT" default:"8080"`

	DBHost string `envconfig:"DB_HOST"`
	DBPort string `envconfig:"DB_PORT"`
	DBUser string `envconfig:"DB_USER"`
	DBPass string `envconfig:"DB_PASSWORD"`
	DBName string `envconfig:"DB_NAME"`

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

	return &cfg, nil
}
