package config_test

import (
	"strings"
	"testing"
	"time"

	"github.com/MorganKryze/FabDoYouMeme/backend/internal/config"
)

// validBaseline is the set of env vars required for a successful Load().
// Individual tests set this first, then override one variable to exercise
// a single failure mode. Keep it in one place so a new required var only
// needs to be added once.
func validBaseline() map[string]string {
	return map[string]string{
		"DATABASE_URL":      "postgres://x:y@localhost/db",
		"RUSTFS_ENDPOINT":   "http://rustfs",
		"RUSTFS_ACCESS_KEY": "key",
		"RUSTFS_SECRET_KEY": "secret",
		"FRONTEND_URL":      "http://localhost:3000",
		"BACKEND_URL":       "http://localhost:8080",
		"SMTP_HOST":         "smtp.example.com",
		"SMTP_FROM":         "noreply@example.com",
	}
}

func applyEnv(t *testing.T, env map[string]string) {
	t.Helper()
	for k, v := range env {
		t.Setenv(k, v)
	}
}

func TestLoad_RequiredMissing(t *testing.T) {
	// Ensure DATABASE_URL is absent — sufficient to trigger required-var check.
	t.Setenv("DATABASE_URL", "")
	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error when required vars missing")
	}
}

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x:y@localhost/db")
	t.Setenv("RUSTFS_ENDPOINT", "http://rustfs")
	t.Setenv("RUSTFS_ACCESS_KEY", "key")
	t.Setenv("RUSTFS_SECRET_KEY", "secret")
	t.Setenv("FRONTEND_URL", "http://localhost:3000")
	t.Setenv("BACKEND_URL", "http://localhost:8080")
	t.Setenv("SMTP_HOST", "smtp.example.com")
	t.Setenv("SMTP_USERNAME", "user")
	t.Setenv("SMTP_PASSWORD", "pass")
	t.Setenv("SMTP_FROM", "noreply@example.com")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != "8080" {
		t.Errorf("default port: got %s", cfg.Port)
	}
	if cfg.SessionTTL != 720*time.Hour {
		t.Errorf("default session TTL: got %v", cfg.SessionTTL)
	}
	if cfg.MagicLinkTTL != 15*time.Minute {
		t.Errorf("default magic link TTL: got %v", cfg.MagicLinkTTL)
	}
	if cfg.SMTPPort != 587 {
		t.Errorf("default SMTP port: got %d", cfg.SMTPPort)
	}
}

// TestConfig_LoadRejectsBadFrontendURL is the P2.8 acceptance test for
// finding 4.D. Pre-fix the `url.Parse` error branch was absent, so any
// malformed FRONTEND_URL silently produced an empty CookieDomain and
// broke login without any startup signal. After the fix each bad form
// fails Load with a specific error.
func TestConfig_LoadRejectsBadFrontendURL(t *testing.T) {
	cases := []struct {
		name      string
		value     string
		wantInErr string
	}{
		{"bare hostname no scheme", "meme.example.com", "scheme and host"},
		{"scheme only", "https://", "scheme and host"},
		{"invalid percent escape (parse error)", "http://%", "not a valid URL"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := validBaseline()
			env["FRONTEND_URL"] = tc.value
			applyEnv(t, env)

			_, err := config.Load()
			if err == nil {
				t.Fatalf("want error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantInErr) {
				t.Fatalf("want error containing %q, got %q", tc.wantInErr, err.Error())
			}
		})
	}
}

// TestConfig_LoadRejectsOutOfBounds is the P2.8 acceptance test for
// finding 4.E. Each subcase overrides exactly one duration/int env var
// with a value outside its documented bounds — the goal is to prove the
// bounds table in validateBounds() actually rejects the realistic
// misconfigurations listed in the review, not just to exercise the
// validator for its own sake.
func TestConfig_LoadRejectsOutOfBounds(t *testing.T) {
	cases := []struct {
		name, key, value, wantInErr string
	}{
		// Durations (negative / zero / absurdly large all forbidden).
		{"session ttl zero", "SESSION_TTL", "0s", "SESSION_TTL"},
		{"session ttl negative", "SESSION_TTL", "-1h", "SESSION_TTL"},
		{"magic link ttl too short", "MAGIC_LINK_TTL", "1ns", "MAGIC_LINK_TTL"},
		{"magic link ttl too long", "MAGIC_LINK_TTL", "720h", "MAGIC_LINK_TTL"},
		{"reconnect grace negative", "RECONNECT_GRACE_WINDOW", "-10s", "RECONNECT_GRACE_WINDOW"},
		{"ws read deadline zero", "WS_READ_DEADLINE", "0s", "WS_READ_DEADLINE"},
		{"ws ping interval zero", "WS_PING_INTERVAL", "0s", "WS_PING_INTERVAL"},
		{"session renew interval zero", "SESSION_RENEW_INTERVAL", "0s", "SESSION_RENEW_INTERVAL"},

		// Ints with min=1 (zero forbidden → rate.Every panic, etc.).
		{"ws rate limit zero", "WS_RATE_LIMIT", "0", "WS_RATE_LIMIT"},
		{"rate limit auth zero", "RATE_LIMIT_AUTH_RPM", "0", "RATE_LIMIT_AUTH_RPM"},
		{"rate limit global negative", "RATE_LIMIT_GLOBAL_RPM", "-1", "RATE_LIMIT_GLOBAL_RPM"},
		{"smtp port out of range", "SMTP_PORT", "70000", "SMTP_PORT"},
		{"smtp port zero", "SMTP_PORT", "0", "SMTP_PORT"},

		// Byte sizes (zero and absurd upper bound).
		{"max upload zero", "MAX_UPLOAD_SIZE_BYTES", "0", "MAX_UPLOAD_SIZE_BYTES"},
		{"ws read limit too small", "WS_READ_LIMIT_BYTES", "1", "WS_READ_LIMIT_BYTES"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := validBaseline()
			env[tc.key] = tc.value
			applyEnv(t, env)

			_, err := config.Load()
			if err == nil {
				t.Fatalf("want error for %s=%s, got nil", tc.key, tc.value)
			}
			if !strings.Contains(err.Error(), tc.wantInErr) {
				t.Fatalf("want error containing %q, got %q", tc.wantInErr, err.Error())
			}
		})
	}
}

// TestConfig_LoadAcceptsBoundaryValues locks in the edges of the bounds
// table. If a future edit narrows a bound, this test catches it before
// the change lands — otherwise we would only find out when an operator's
// existing deploy fails to start.
func TestConfig_LoadAcceptsBoundaryValues(t *testing.T) {
	env := validBaseline()
	env["SESSION_TTL"] = "1m"               // min
	env["MAGIC_LINK_TTL"] = "24h"           // max
	env["RECONNECT_GRACE_WINDOW"] = "1s"    // min
	env["WS_READ_DEADLINE"] = "5s"          // min
	env["WS_PING_INTERVAL"] = "5m"          // max
	env["SESSION_RENEW_INTERVAL"] = "1m"    // min
	env["WS_RATE_LIMIT"] = "1"              // min
	env["SMTP_PORT"] = "65535"              // max
	env["WS_READ_LIMIT_BYTES"] = "64"       // min
	env["MAX_UPLOAD_SIZE_BYTES"] = "1"      // min
	env["RATE_LIMIT_AUTH_RPM"] = "1"        // min
	env["RATE_LIMIT_GLOBAL_RPM"] = "100000" // max
	applyEnv(t, env)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("boundary values rejected: %v", err)
	}
	if cfg.SessionTTL != time.Minute {
		t.Errorf("boundary SessionTTL round-trip: got %v", cfg.SessionTTL)
	}
}
