package config_test

import (
	"os"
	"testing"
	"time"

	"github.com/MorganKryze/FabDoYouMeme/backend/internal/config"
)

func TestLoad_RequiredMissing(t *testing.T) {
	os.Clearenv()
	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error when required vars missing")
	}
}

func TestLoad_Defaults(t *testing.T) {
	os.Clearenv()
	os.Setenv("DATABASE_URL", "postgres://x:y@localhost/db")
	os.Setenv("RUSTFS_ENDPOINT", "http://rustfs")
	os.Setenv("RUSTFS_ACCESS_KEY", "key")
	os.Setenv("RUSTFS_SECRET_KEY", "secret")
	os.Setenv("FRONTEND_URL", "http://localhost:3000")
	os.Setenv("BACKEND_URL", "http://localhost:8080")
	os.Setenv("SMTP_HOST", "smtp.example.com")
	os.Setenv("SMTP_USERNAME", "user")
	os.Setenv("SMTP_PASSWORD", "pass")
	os.Setenv("SMTP_FROM", "noreply@example.com")

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
