package config

import (
	"os"
	"testing"
	"time"
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
	os.Setenv("JWT_ISSUER", "api-service")
	os.Setenv("JWT_AUDIENCE", "client-app")
	os.Setenv("ACCESS_TOKEN_TTL", "30m")
	os.Setenv("REFRESH_TOKEN_TTL", "720h")

	defer func() {
		os.Unsetenv("APP_PORT")
		os.Unsetenv("DB_HOST")
		os.Unsetenv("DB_PORT")
		os.Unsetenv("DB_USER")
		os.Unsetenv("DB_PASSWORD")
		os.Unsetenv("DB_NAME")
		os.Unsetenv("JWT_SECRET")
		os.Unsetenv("JWT_ISSUER")
		os.Unsetenv("JWT_AUDIENCE")
		os.Unsetenv("ACCESS_TOKEN_TTL")
		os.Unsetenv("REFRESH_TOKEN_TTL")
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
	if cfg.JWTIssuer != "api-service" {
		t.Errorf("expected JWTIssuer api-service, got %s", cfg.JWTIssuer)
	}
	if cfg.JWTAudience != "client-app" {
		t.Errorf("expected JWTAudience client-app, got %s", cfg.JWTAudience)
	}
	if cfg.AccessTokenTTL != 30*time.Minute {
		t.Errorf("expected AccessTokenTTL 30m, got %s", cfg.AccessTokenTTL)
	}
	if cfg.RefreshTokenTTL != 720*time.Hour {
		t.Errorf("expected RefreshTokenTTL 720h, got %s", cfg.RefreshTokenTTL)
	}
}
