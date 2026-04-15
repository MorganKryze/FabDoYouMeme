package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/MorganKryze/FabDoYouMeme/backend/internal/middleware"
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

	CookieDomain string

	SeedAdminEmail string
	LogLevel       string

	// AppEnv gates env-specific features. Legal values: "dev", "preprod",
	// "prod". Defaults to "prod" when unset — fail safe, because features
	// that disable in prod should stay disabled if the operator forgets to
	// set it. Consumed by danger-zone route registration in main.go.
	AppEnv string

	ReconnectGraceWindow time.Duration
	WSRateLimit          int
	WSReadLimitBytes     int64
	WSReadDeadline       time.Duration
	WSPingInterval       time.Duration

	MaxUploadSizeBytes int64

	// AllowedOrigins is the normalized set of origins accepted by the
	// WebSocket upgrader. Each entry has whitespace and trailing slashes
	// stripped. Seeded from FRONTEND_URL and the optional comma-separated
	// TRUSTED_WS_ORIGINS env var — see finding 5.C in the 2026-04-10 review
	// for why exact-match on a single string was fragile.
	AllowedOrigins []string

	RateLimitAuthRPM    int
	RateLimitInviteRPH  int
	RateLimitRoomsRPH   int
	RateLimitUploadsRPH int
	RateLimitGlobalRPM  int

	// TrustedProxies is the parsed CIDR allowlist consumed by
	// middleware.ClientIP. Empty means "treat all connections as direct" —
	// the safe default for deployments without a reverse proxy.
	TrustedProxies []*net.IPNet
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
		AppEnv:           getEnv("APP_ENV", "prod"),
	}

	// Fail-loud on bad FRONTEND_URL (finding 4.D). url.Parse accepts almost
	// anything — most notably a bare hostname like "meme.example.com" parses
	// successfully but yields Hostname()=="", silently leaving CookieDomain
	// empty and breaking cross-subdomain login. The explicit scheme+host
	// check catches both url.Parse errors and the silent-zero cases.
	u, err := url.Parse(cfg.FrontendURL)
	if err != nil {
		return nil, fmt.Errorf("FRONTEND_URL is not a valid URL: %w", err)
	}
	if u.Scheme == "" || u.Hostname() == "" {
		return nil, fmt.Errorf("FRONTEND_URL must include a scheme and host, got %q", cfg.FrontendURL)
	}
	cfg.CookieDomain = u.Hostname()
	cfg.AllowedOrigins = buildAllowedOrigins(cfg.FrontendURL, os.Getenv("TRUSTED_WS_ORIGINS"))

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

	// TRUSTED_PROXIES is comma-separated CIDR ranges or bare IPs (the latter
	// are promoted to /32 or /128). Required only when running behind a
	// reverse proxy that forwards X-Forwarded-For — without it, ClientIP
	// falls back to r.RemoteAddr, which is the safe direct-connection mode.
	if cfg.TrustedProxies, err = middleware.ParseTrustedProxies(os.Getenv("TRUSTED_PROXIES")); err != nil {
		return nil, fmt.Errorf("TRUSTED_PROXIES: %w", err)
	}

	if err := cfg.validateBounds(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// validateBounds rejects out-of-range duration/int config values (finding
// 4.E). Negative or zero durations silently break core flows — e.g.
// SESSION_TTL=0 logs users out before their verify response returns, and
// WS_RATE_LIMIT=0 panics `rate.Every`. The bounds here are intentionally
// generous: the goal is to reject obviously broken misconfigurations, not
// to second-guess the operator.
func (cfg *Config) validateBounds() error {
	durationBound := func(name string, v, min, max time.Duration) error {
		if v < min || v > max {
			return fmt.Errorf("%s=%v must be between %v and %v", name, v, min, max)
		}
		return nil
	}
	intBound := func(name string, v, min, max int) error {
		if v < min || v > max {
			return fmt.Errorf("%s=%d must be between %d and %d", name, v, min, max)
		}
		return nil
	}
	int64Bound := func(name string, v, min, max int64) error {
		if v < min || v > max {
			return fmt.Errorf("%s=%d must be between %d and %d", name, v, min, max)
		}
		return nil
	}

	checks := []error{
		durationBound("MAGIC_LINK_TTL", cfg.MagicLinkTTL, 30*time.Second, 24*time.Hour),
		durationBound("SESSION_TTL", cfg.SessionTTL, 1*time.Minute, 365*24*time.Hour),
		durationBound("SESSION_RENEW_INTERVAL", cfg.SessionRenewInterval, 1*time.Minute, 24*time.Hour),
		durationBound("RECONNECT_GRACE_WINDOW", cfg.ReconnectGraceWindow, 1*time.Second, 10*time.Minute),
		durationBound("WS_READ_DEADLINE", cfg.WSReadDeadline, 5*time.Second, 10*time.Minute),
		durationBound("WS_PING_INTERVAL", cfg.WSPingInterval, 1*time.Second, 5*time.Minute),

		intBound("SMTP_PORT", cfg.SMTPPort, 1, 65535),
		intBound("WS_RATE_LIMIT", cfg.WSRateLimit, 1, 10000),
		intBound("RATE_LIMIT_AUTH_RPM", cfg.RateLimitAuthRPM, 1, 100000),
		intBound("RATE_LIMIT_INVITE_VALIDATION_RPH", cfg.RateLimitInviteRPH, 1, 100000),
		intBound("RATE_LIMIT_ROOMS_RPH", cfg.RateLimitRoomsRPH, 1, 100000),
		intBound("RATE_LIMIT_UPLOADS_RPH", cfg.RateLimitUploadsRPH, 1, 100000),
		intBound("RATE_LIMIT_GLOBAL_RPM", cfg.RateLimitGlobalRPM, 1, 100000),

		int64Bound("WS_READ_LIMIT_BYTES", cfg.WSReadLimitBytes, 64, 1_048_576),
		int64Bound("MAX_UPLOAD_SIZE_BYTES", cfg.MaxUploadSizeBytes, 1, 104_857_600),

		func() error {
			switch cfg.AppEnv {
			case "dev", "preprod", "prod":
				return nil
			default:
				return fmt.Errorf("APP_ENV=%q must be one of dev, preprod, prod", cfg.AppEnv)
			}
		}(),
	}
	for _, e := range checks {
		if e != nil {
			return e
		}
	}
	return nil
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

// NormalizeOrigin trims surrounding whitespace and a single trailing slash so
// that exact-match comparison in the WS upgrader tolerates the browser
// variants noted in finding 5.C. Exported so ws.go and tests share one rule.
func NormalizeOrigin(s string) string {
	return strings.TrimRight(strings.TrimSpace(s), "/")
}

// buildAllowedOrigins derives the WS origin allowlist from FRONTEND_URL plus
// an optional comma-separated TRUSTED_WS_ORIGINS override. Duplicates and
// blank entries are dropped so callers can iterate without guarding.
func buildAllowedOrigins(frontendURL, extraCSV string) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0, 2)
	add := func(s string) {
		n := NormalizeOrigin(s)
		if n == "" {
			return
		}
		if _, dup := seen[n]; dup {
			return
		}
		seen[n] = struct{}{}
		out = append(out, n)
	}
	add(frontendURL)
	for _, piece := range strings.Split(extraCSV, ",") {
		add(piece)
	}
	return out
}
