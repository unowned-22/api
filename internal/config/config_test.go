package config

import (
	"os"
	"testing"
)

func TestConfigLoad(t *testing.T) {
	// Setup env variables for test
	os.Setenv("APP_PORT", "9999")
	os.Setenv("DB_HOST", "somehost")
	os.Setenv("DB_PORT", "1234")
	os.Setenv("DB_USER", "someuser")
	os.Setenv("DB_PASSWORD", "somepass")
	os.Setenv("DB_NAME", "somedb")
	os.Setenv("JWT_SECRET", "somesecret")

	defer func() {
		os.Unsetenv("APP_PORT")
		os.Unsetenv("DB_HOST")
		os.Unsetenv("DB_PORT")
		os.Unsetenv("DB_USER")
		os.Unsetenv("DB_PASSWORD")
		os.Unsetenv("DB_NAME")
		os.Unsetenv("JWT_SECRET")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.AppPort != "9999" {
		t.Errorf("expected AppPort 9999, got %s", cfg.AppPort)
	}
	if cfg.DBHost != "somehost" {
		t.Errorf("expected DBHost somehost, got %s", cfg.DBHost)
	}
	if cfg.DBPass != "somepass" {
		t.Errorf("expected DBPass somepass, got %s", cfg.DBPass)
	}
}
