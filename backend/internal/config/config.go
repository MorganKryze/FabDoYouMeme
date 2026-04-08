package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	DatabaseURL     string
	Port            string
	FrontendURL     string
	BackendURL      string
	RustFSEndpoint  string
	RustFSAccessKey string
	RustFSSecretKey string
	RustFSBucket    string

	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
	SMTPFrom     string

	MagicLinkTTL         time.Duration
	MagicLinkBaseURL     string
	SessionTTL           time.Duration
	SessionRenewInterval time.Duration

	SeedAdminEmail string
	LogLevel       string

	ReconnectGraceWindow time.Duration
	WSRateLimit          int
	WSReadLimitBytes     int64
	WSReadDeadline       time.Duration
	WSPingInterval       time.Duration

	MaxUploadSizeBytes int64

	RateLimitAuthRPM    int
	RateLimitInviteRPH  int
	RateLimitRoomsRPH   int
	RateLimitUploadsRPH int
	RateLimitGlobalRPM  int
}

func Load() (*Config, error) {
	required := []string{
		"DATABASE_URL", "RUSTFS_ENDPOINT", "RUSTFS_ACCESS_KEY", "RUSTFS_SECRET_KEY",
		"FRONTEND_URL", "BACKEND_URL", "SMTP_HOST", "SMTP_FROM",
	}
	for _, k := range required {
		if os.Getenv(k) == "" {
			return nil, fmt.Errorf("required env var %s is not set", k)
		}
	}

	cfg := &Config{
		DatabaseURL:      os.Getenv("DATABASE_URL"),
		Port:             getEnv("PORT", "8080"),
		FrontendURL:      os.Getenv("FRONTEND_URL"),
		BackendURL:       os.Getenv("BACKEND_URL"),
		RustFSEndpoint:   os.Getenv("RUSTFS_ENDPOINT"),
		RustFSAccessKey:  os.Getenv("RUSTFS_ACCESS_KEY"),
		RustFSSecretKey:  os.Getenv("RUSTFS_SECRET_KEY"),
		RustFSBucket:     getEnv("RUSTFS_BUCKET", "fabyoumeme-assets"),
		SMTPHost:         os.Getenv("SMTP_HOST"),
		SMTPUsername:     os.Getenv("SMTP_USERNAME"),
		SMTPPassword:     os.Getenv("SMTP_PASSWORD"),
		SMTPFrom:         os.Getenv("SMTP_FROM"),
		MagicLinkBaseURL: os.Getenv("FRONTEND_URL"),
		SeedAdminEmail:   os.Getenv("SEED_ADMIN_EMAIL"),
		LogLevel:         getEnv("LOG_LEVEL", "info"),
	}

	var err error
	if cfg.SMTPPort, err = getEnvInt("SMTP_PORT", 587); err != nil {
		return nil, fmt.Errorf("SMTP_PORT: %w", err)
	}
	if cfg.MagicLinkTTL, err = getEnvDuration("MAGIC_LINK_TTL", 15*time.Minute); err != nil {
		return nil, fmt.Errorf("MAGIC_LINK_TTL: %w", err)
	}
	if cfg.SessionTTL, err = getEnvDuration("SESSION_TTL", 720*time.Hour); err != nil {
		return nil, fmt.Errorf("SESSION_TTL: %w", err)
	}
	if cfg.SessionRenewInterval, err = getEnvDuration("SESSION_RENEW_INTERVAL", 60*time.Minute); err != nil {
		return nil, fmt.Errorf("SESSION_RENEW_INTERVAL: %w", err)
	}
	if cfg.ReconnectGraceWindow, err = getEnvDuration("RECONNECT_GRACE_WINDOW", 30*time.Second); err != nil {
		return nil, fmt.Errorf("RECONNECT_GRACE_WINDOW: %w", err)
	}
	if cfg.WSReadDeadline, err = getEnvDuration("WS_READ_DEADLINE", 60*time.Second); err != nil {
		return nil, fmt.Errorf("WS_READ_DEADLINE: %w", err)
	}
	if cfg.WSPingInterval, err = getEnvDuration("WS_PING_INTERVAL", 25*time.Second); err != nil {
		return nil, fmt.Errorf("WS_PING_INTERVAL: %w", err)
	}
	if cfg.WSRateLimit, err = getEnvInt("WS_RATE_LIMIT", 20); err != nil {
		return nil, fmt.Errorf("WS_RATE_LIMIT: %w", err)
	}
	if cfg.WSReadLimitBytes, err = getEnvInt64("WS_READ_LIMIT_BYTES", 4096); err != nil {
		return nil, fmt.Errorf("WS_READ_LIMIT_BYTES: %w", err)
	}
	if cfg.MaxUploadSizeBytes, err = getEnvInt64("MAX_UPLOAD_SIZE_BYTES", 2097152); err != nil {
		return nil, fmt.Errorf("MAX_UPLOAD_SIZE_BYTES: %w", err)
	}
	if cfg.RateLimitAuthRPM, err = getEnvInt("RATE_LIMIT_AUTH_RPM", 10); err != nil {
		return nil, fmt.Errorf("RATE_LIMIT_AUTH_RPM: %w", err)
	}
	if cfg.RateLimitInviteRPH, err = getEnvInt("RATE_LIMIT_INVITE_VALIDATION_RPH", 20); err != nil {
		return nil, fmt.Errorf("RATE_LIMIT_INVITE_VALIDATION_RPH: %w", err)
	}
	if cfg.RateLimitRoomsRPH, err = getEnvInt("RATE_LIMIT_ROOMS_RPH", 10); err != nil {
		return nil, fmt.Errorf("RATE_LIMIT_ROOMS_RPH: %w", err)
	}
	if cfg.RateLimitUploadsRPH, err = getEnvInt("RATE_LIMIT_UPLOADS_RPH", 50); err != nil {
		return nil, fmt.Errorf("RATE_LIMIT_UPLOADS_RPH: %w", err)
	}
	if cfg.RateLimitGlobalRPM, err = getEnvInt("RATE_LIMIT_GLOBAL_RPM", 100); err != nil {
		return nil, fmt.Errorf("RATE_LIMIT_GLOBAL_RPM: %w", err)
	}
	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	return strconv.Atoi(v)
}

func getEnvInt64(key string, fallback int64) (int64, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	return strconv.ParseInt(v, 10, 64)
}

func getEnvDuration(key string, fallback time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	return time.ParseDuration(v)
}
