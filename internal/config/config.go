package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	AppPort            string `envconfig:"APP_PORT"`
	AppEnv             string `envconfig:"APP_ENV"`
	CORSAllowedOrigins string `envconfig:"CORS_ALLOWED_ORIGINS"`

	RateLimitRPS       float64 `envconfig:"RATE_LIMIT_RPS" default:"10"`
	RateLimitBurst     int     `envconfig:"RATE_LIMIT_BURST" default:"20"`
	AuthRateLimitRPS   float64 `envconfig:"AUTH_RATE_LIMIT_RPS" default:"3"`
	AuthRateLimitBurst int     `envconfig:"AUTH_RATE_LIMIT_BURST" default:"5"`

	SMTPHost     string `envconfig:"SMTP_HOST" default:"localhost"`
	SMTPPort     int    `envconfig:"SMTP_PORT" default:"1025"`
	SMTPUsername string `envconfig:"SMTP_USERNAME" default:""`
	SMTPPassword string `envconfig:"SMTP_PASSWORD" default:""`
	SMTPFrom     string `envconfig:"SMTP_FROM" default:"No Reply <noreply@example.com>"`
	AppName      string `envconfig:"APP_NAME" default:"App"`
	AppURL       string `envconfig:"APP_URL" default:"http://localhost:3222"`

	RabbitMQURL                          string `envconfig:"RABBITMQ_URL"`
	RabbitMQExchange                     string `envconfig:"RABBITMQ_EXCHANGE" default:"app.events"`
	RabbitMQQueue                        string `envconfig:"RABBITMQ_QUEUE"    default:"app.worker"`
	RabbitMQRealtimeQueue                string `envconfig:"RABBITMQ_REALTIME_QUEUE" default:"app.realtime"`
	RabbitMQDeadLetterExchange           string `envconfig:"RABBITMQ_DLX"             default:"app.dlx"`
	RabbitMQDeadLetterRoutingKey         string `envconfig:"RABBITMQ_DLX_ROUTING_KEY" default:"app.worker.dead"`
	RabbitMQRealtimeDeadLetterRoutingKey string `envconfig:"RABBITMQ_REALTIME_DLX_ROUTING_KEY" default:"app.realtime.dead"`

	DBHost    string `envconfig:"DB_HOST"`
	DBPort    string `envconfig:"DB_PORT"`
	DBUser    string `envconfig:"DB_USER"`
	DBPass    string `envconfig:"DB_PASSWORD"`
	DBName    string `envconfig:"DB_NAME"`
	DBSSLMode string `envconfig:"DB_SSL_MODE" default:"disable"`

	MinIOEndpoint         string `envconfig:"MINIO_ENDPOINT"`
	MinIOAccessKey        string `envconfig:"MINIO_ACCESS_KEY"`
	MinIOSecretKey        string `envconfig:"MINIO_SECRET_KEY"`
	MinIOUseSSL           bool   `envconfig:"MINIO_USE_SSL"`
	MinIORegion           string `envconfig:"MINIO_REGION"`
	MinIOBucket           string `envconfig:"MINIO_BUCKET"`
	StoragePublicEndpoint string `envconfig:"STORAGE_PUBLIC_ENDPOINT"`

	MeilisearchHost   string `envconfig:"MEILISEARCH_HOST"`
	MeilisearchAPIKey string `envconfig:"MEILISEARCH_API_KEY"`

	JWTSecret       string        `envconfig:"JWT_SECRET"`
	JWTIssuer       string        `envconfig:"JWT_ISSUER"`
	JWTAudience     string        `envconfig:"JWT_AUDIENCE"`
	AccessTokenTTL  time.Duration `envconfig:"ACCESS_TOKEN_TTL"  default:"15m"`
	RefreshTokenTTL time.Duration `envconfig:"REFRESH_TOKEN_TTL" default:"720h"`
	RedisURL        string        `envconfig:"REDIS_URL" default:""`

	LoginRateLimit                    int           `envconfig:"LOGIN_RATE_LIMIT"                      default:"5"`
	LoginRateLimitWindow              time.Duration `envconfig:"LOGIN_RATE_LIMIT_WINDOW"               default:"5m"`
	RegisterRateLimit                 int           `envconfig:"REGISTER_RATE_LIMIT"                   default:"3"`
	RegisterRateLimitWindow           time.Duration `envconfig:"REGISTER_RATE_LIMIT_WINDOW"            default:"1h"`
	ForgotPasswordRateLimit           int           `envconfig:"FORGOT_PASSWORD_RATE_LIMIT"            default:"3"`
	ForgotPasswordRateLimitWindow     time.Duration `envconfig:"FORGOT_PASSWORD_RATE_LIMIT_WINDOW"     default:"15m"`
	ResendVerificationRateLimit       int           `envconfig:"RESEND_VERIFICATION_RATE_LIMIT"        default:"3"`
	ResendVerificationRateLimitWindow time.Duration `envconfig:"RESEND_VERIFICATION_RATE_LIMIT_WINDOW" default:"15m"`

	StoriesCleanupIntervalMinutes int `envconfig:"STORIES_CLEANUP_INTERVAL_MINUTES" default:"10"`

	MessengerMaxFileSizeBytes  int64  `envconfig:"MESSENGER_MAX_FILE_SIZE_BYTES"    default:"52428800"`
	MessengerMaxAudioDurationS int    `envconfig:"MESSENGER_MAX_AUDIO_DURATION_S"   default:"300"`
	MessengerMaxGroupMembers   int    `envconfig:"MESSENGER_MAX_GROUP_MEMBERS"      default:"500"`
	MessengerTypingTimeoutS    int    `envconfig:"MESSENGER_TYPING_TIMEOUT_S"       default:"5"`
	MessengerInviteLinkBaseURL string `envconfig:"MESSENGER_INVITE_LINK_BASE_URL"`
	MessengerDefaultDisappearS int    `envconfig:"MESSENGER_DEFAULT_DISAPPEAR_S"    default:"0"`

	VideoProcessQueue     string `envconfig:"VIDEO_PROCESS_QUEUE"       default:"video.process"`
	VideoMaxFileSizeBytes int64  `envconfig:"VIDEO_MAX_FILE_SIZE_BYTES" default:"2147483648"`
}

func Load() (*Config, error) {
	_ = godotenv.Load()

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
		"DB_HOST":      c.DBHost,
		"DB_PORT":      c.DBPort,
		"DB_USER":      c.DBUser,
		"DB_NAME":      c.DBName,
		"JWT_SECRET":   c.JWTSecret,
		"JWT_ISSUER":   c.JWTIssuer,
		"JWT_AUDIENCE": c.JWTAudience,
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

	if c.AccessTokenTTL <= 0 {
		return fmt.Errorf("ACCESS_TOKEN_TTL must be greater than zero")
	}
	if c.RefreshTokenTTL <= 0 {
		return fmt.Errorf("REFRESH_TOKEN_TTL must be greater than zero")
	}

	if c.LoginRateLimit <= 0 {
		return fmt.Errorf("LOGIN_RATE_LIMIT must be greater than zero")
	}
	if c.LoginRateLimitWindow <= 0 {
		return fmt.Errorf("LOGIN_RATE_LIMIT_WINDOW must be greater than zero")
	}
	if c.RegisterRateLimit <= 0 {
		return fmt.Errorf("REGISTER_RATE_LIMIT must be greater than zero")
	}
	if c.RegisterRateLimitWindow <= 0 {
		return fmt.Errorf("REGISTER_RATE_LIMIT_WINDOW must be greater than zero")
	}
	if c.ForgotPasswordRateLimit <= 0 {
		return fmt.Errorf("FORGOT_PASSWORD_RATE_LIMIT must be greater than zero")
	}
	if c.ForgotPasswordRateLimitWindow <= 0 {
		return fmt.Errorf("FORGOT_PASSWORD_RATE_LIMIT_WINDOW must be greater than zero")
	}
	if c.ResendVerificationRateLimit <= 0 {
		return fmt.Errorf("RESEND_VERIFICATION_RATE_LIMIT must be greater than zero")
	}
	if c.ResendVerificationRateLimitWindow <= 0 {
		return fmt.Errorf("RESEND_VERIFICATION_RATE_LIMIT_WINDOW must be greater than zero")
	}

	return nil
}
